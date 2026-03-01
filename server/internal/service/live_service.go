// Package service contains the core business logic for live streaming orchestration.
package service

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/maxence2997/delta-cast/server/internal/logger"

	"github.com/google/uuid"
	"github.com/maxence2997/delta-cast/server/internal/model"
	"github.com/maxence2997/delta-cast/server/internal/provider"
)

// RelayOptions controls which relay targets are active.
// Add a new field here when a third relay target is introduced;
// the NewLiveService signature does not need to change.
type RelayOptions struct {
	// GCPRelayEnabled enables the GCP Live Stream relay target.
	// When false, all GCP API calls are skipped and GCP_* env vars are not required.
	GCPRelayEnabled bool
	// YouTubeRelayEnabled enables the YouTube relay target.
	// When false, all YouTube API calls are skipped and YOUTUBE_* env vars are not required.
	YouTubeRelayEnabled bool
}

// LiveService orchestrates the live streaming session lifecycle.
type LiveService struct {
	mu sync.Mutex

	session     *model.Session
	allocCancel context.CancelFunc // cancels in-flight allocateResources; nil when idle

	relay RelayOptions

	agoraToken     provider.AgoraTokenProvider
	agoraMediaPush provider.AgoraMediaPushProvider
	gcp            provider.GCPLiveStreamProvider
	youtube        provider.YouTubeProvider
}

// newIdleSession returns a fresh idle session with all deduplication structures initialized.
func newIdleSession() *model.Session {
	return &model.Session{
		State:         model.StateIdle,
		SeenNoticeIDs: make(map[string]struct{}),
	}
}

// NewLiveService creates a new LiveService with the provided providers.
// opts controls which relay targets are active; set a field to false to skip
// that provider entirely (useful for debugging).
func NewLiveService(
	agoraToken provider.AgoraTokenProvider,
	agoraMediaPush provider.AgoraMediaPushProvider,
	gcp provider.GCPLiveStreamProvider,
	youtube provider.YouTubeProvider,
	opts RelayOptions,
) *LiveService {
	return &LiveService{
		session:        newIdleSession(),
		relay:          opts,
		agoraToken:     agoraToken,
		agoraMediaPush: agoraMediaPush,
		gcp:            gcp,
		youtube:        youtube,
	}
}

// Prepare pre-warms GCP and YouTube resources. This is an async-style operation
// that transitions the session from idle → preparing → ready.
func (s *LiveService) Prepare(ctx context.Context) (*model.PrepareResponse, error) {
	s.mu.Lock()

	// If already preparing or beyond, return existing session info
	if s.session.State != model.StateIdle {
		resp := &model.PrepareResponse{
			SessionID: s.session.ID,
			State:     s.session.State,
			Message:   "session already exists",
		}
		s.mu.Unlock()
		return resp, nil
	}

	// Create new session
	sessionID := uuid.New().String()[:8]
	s.session = &model.Session{
		ID:            sessionID,
		State:         model.StatePreparing,
		CreatedAt:     time.Now(),
		AgoraChannel:  fmt.Sprintf("deltacast-%s", sessionID),
		SeenNoticeIDs: make(map[string]struct{}),
	}
	s.mu.Unlock()

	// Run resource allocation in background
	go s.allocateResources(sessionID)

	return &model.PrepareResponse{
		SessionID: sessionID,
		State:     model.StatePreparing,
		Message:   "resource allocation started, poll /v1/live/status for updates",
	}, nil
}

// allocateResources creates GCP and YouTube resources in parallel.
// It is always launched as a goroutine by Prepare.
// If Stop is called mid-flight, allocCancel is invoked, which causes all
// in-flight provider calls to return early; this function then performs
// a best-effort cleanup of any resources that were already created.
func (s *LiveService) allocateResources(sessionID string) {
	logger.Infof("[prepare] starting resource allocation for session %s", sessionID)
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)

	s.mu.Lock()
	s.allocCancel = cancel
	s.mu.Unlock()

	defer func() {
		cancel()
		s.mu.Lock()
		s.allocCancel = nil
		s.mu.Unlock()
	}()

	inputID := fmt.Sprintf("input-%s", sessionID)
	channelID := fmt.Sprintf("channel-%s", sessionID)

	var (
		gcpErr error
		ytErr  error
		wg     sync.WaitGroup
	)

	// GCP: Create Input → Create Channel → Start Channel → Wait for ready
	var gcpInputURI string
	if s.relay.GCPRelayEnabled {
		wg.Go(func() {
			var err error
			gcpInputURI, err = s.setupGCPResources(ctx, inputID, channelID)
			if err != nil {
				gcpErr = err
			}
		})
	}

	// YouTube: Create Broadcast → Create Stream → Bind
	var (
		broadcastID string
		streamID    string
		rtmpURL     string
		streamKey   string
	)
	if s.relay.YouTubeRelayEnabled {
		wg.Go(func() {
			var err error
			broadcastID, streamID, rtmpURL, streamKey, err = s.setupYouTubeResources(ctx, sessionID)
			if err != nil {
				ytErr = err
			}
		})
	}

	wg.Wait()
	logger.Infof("[prepare] resource allocation finished for session %s", sessionID)

	s.mu.Lock()
	sessionChanged := s.session.ID != sessionID
	stateStopping := s.session.State == model.StateStopping
	s.mu.Unlock()

	// If Stop was called while we were allocating, clean up any partial resources
	// using a fresh context (the original ctx may already be cancelled).
	if sessionChanged || stateStopping {
		logger.Infof("[prepare] session %s was interrupted mid-allocation, cleaning up partial resources", sessionID)
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cleanupCancel()
		if s.relay.GCPRelayEnabled {
			// Attempt stop+delete even if only partially created; 404s are ignored by the provider.
			if err := s.gcp.StopChannel(cleanupCtx, channelID); err != nil {
				logger.Warnf("[prepare] cleanup: stop channel: %v", err)
			}
			if err := s.gcp.DeleteChannel(cleanupCtx, channelID); err != nil {
				logger.Warnf("[prepare] cleanup: delete channel: %v", err)
			}
			if err := s.gcp.DeleteInput(cleanupCtx, inputID); err != nil {
				logger.Warnf("[prepare] cleanup: delete input: %v", err)
			}
		}
		if s.relay.YouTubeRelayEnabled && broadcastID != "" {
			if err := s.youtube.TransitionBroadcast(cleanupCtx, broadcastID, "complete"); err != nil {
				logger.Warnf("[prepare] cleanup: youtube transition: %v", err)
			}
		}
		s.mu.Lock()
		if s.session.ID == sessionID {
			s.session = newIdleSession()
		}
		s.mu.Unlock()
		logger.Infof("[prepare] cleanup complete for interrupted session %s", sessionID)
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if gcpErr != nil {
		logger.Errorf("resource allocation failed (GCP): %v", gcpErr)
		s.session.State = model.StateIdle
		return
	}
	if ytErr != nil {
		logger.Errorf("resource allocation failed (YouTube): %v", ytErr)
		s.session.State = model.StateIdle
		return
	}

	// Update session with resource details
	if s.relay.GCPRelayEnabled {
		s.session.GCPInputID = inputID
		s.session.GCPChannelID = channelID
		s.session.GCPInputURI = gcpInputURI
		s.session.GCPPlaybackURL = s.gcp.GetPlaybackURL(channelID)
	}
	if s.relay.YouTubeRelayEnabled {
		s.session.YouTubeBroadcastID = broadcastID
		s.session.YouTubeStreamID = streamID
		s.session.YouTubeStreamKey = streamKey
		s.session.YouTubeRTMPURL = fmt.Sprintf("%s/%s", rtmpURL, streamKey)
		s.session.YouTubeWatchURL = s.youtube.GetWatchURL(broadcastID)
	}
	s.session.State = model.StateReady

	logger.Infof("session %s resources ready (gcp=%v, youtube=%v)", sessionID, s.relay.GCPRelayEnabled, s.relay.YouTubeRelayEnabled)
}

// Start returns an Agora token for the client to begin streaming.
// Resources must already be in ready state (from Prepare).
func (s *LiveService) Start(ctx context.Context, appID string) (*model.StartResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.session.State == model.StateLive {
		// Already live — return existing info (idempotent)
		token, err := s.agoraToken.GenerateRTCToken(s.session.AgoraChannel, 0, 86400)
		if err != nil {
			return nil, fmt.Errorf("generate token: %w", err)
		}
		return &model.StartResponse{
			SessionID:    s.session.ID,
			AgoraAppID:   appID,
			AgoraChannel: s.session.AgoraChannel,
			AgoraToken:   token,
			AgoraUID:     0,
		}, nil
	}

	if s.session.State != model.StateReady {
		return nil, fmt.Errorf("session is in %s state, must be ready to start", s.session.State)
	}

	token, err := s.agoraToken.GenerateRTCToken(s.session.AgoraChannel, 0, 86400)
	if err != nil {
		return nil, fmt.Errorf("generate agora token: %w", err)
	}

	return &model.StartResponse{
		SessionID:    s.session.ID,
		AgoraAppID:   appID,
		AgoraChannel: s.session.AgoraChannel,
		AgoraToken:   token,
		AgoraUID:     0,
	}, nil
}

// HandleMediaPushWebhook processes Agora Media Push notification events (productId=5).
// Event types: 1=ConverterCreated, 2=ConverterConfigChanged, 3=ConverterStateChanged, 4=ConverterDestroyed.
// noticeID is used for deduplication: duplicate deliveries of the same notification are silently ignored.
func (s *LiveService) HandleMediaPushWebhook(_ context.Context, noticeID string, eventType int, converterID, converterState, destroyReason string) error {
	if noticeID != "" {
		s.mu.Lock()
		if _, seen := s.session.SeenNoticeIDs[noticeID]; seen {
			s.mu.Unlock()
			logger.Infof("ignoring duplicate media push webhook (noticeId=%s)", noticeID)
			return nil
		}
		s.session.SeenNoticeIDs[noticeID] = struct{}{}
		s.mu.Unlock()
	}
	switch eventType {
	case 1:
		logger.Infof("media push converter created: id=%s", converterID)
	case 2:
		logger.Infof("media push converter config changed: id=%s", converterID)
	case 3:
		switch converterState {
		case "running":
			logger.Infof("media push converter running: id=%s", converterID)
		case "failed":
			logger.Errorf("media push converter failed: id=%s", converterID)
		default:
			logger.Infof("media push converter state changed: id=%s state=%s", converterID, converterState)
		}
	case 4:
		if destroyReason == "Internal Error" {
			logger.Errorf("media push converter destroyed unexpectedly: id=%s reason=%s", converterID, destroyReason)
		} else {
			logger.Infof("media push converter destroyed: id=%s reason=%s", converterID, destroyReason)
		}
	default:
		logger.Infof("media push unknown event type %d: id=%s", eventType, converterID)
	}
	return nil
}

// HandleChannelWebhook processes Agora RTC Channel NCS events (productId=1).
// For event 103 (broadcaster joins channel), it triggers Media Push to both targets.
// noticeID is used for deduplication; uid is the broadcaster's Agora RTC UID;
// channelName is the Agora channel from the NCS payload;
// clientSeq is the sequence number used for broadcaster-join deduplication (0 if unavailable).
func (s *LiveService) HandleChannelWebhook(ctx context.Context, noticeID string, eventType int, uid uint32, channelName string, clientSeq int64) error {
	// Deduplicate by noticeId: Agora may deliver the same notification more than
	// once. Guard before any state changes or logging to keep the log clean.
	if noticeID != "" {
		s.mu.Lock()
		if _, seen := s.session.SeenNoticeIDs[noticeID]; seen {
			s.mu.Unlock()
			logger.Infof("ignoring duplicate channel webhook (noticeId=%s)", noticeID)
			return nil
		}
		s.session.SeenNoticeIDs[noticeID] = struct{}{}
		s.mu.Unlock()
	}
	// Only event 103 (broadcaster join) triggers business logic.
	// All other events are logged for observability and acknowledged with 200 OK.
	if eventType != 103 {
		logger.Infof("received agora channel event %d: channel=%q uid=%d clientSeq=%d (no action taken)", eventType, channelName, uid, clientSeq)
		return nil
	}

	s.mu.Lock()

	// Validate channel name: reject events for a different channel.
	// The NCS health check uses channelName="test_webhook" while the session is idle,
	// so we only validate when the session has an assigned channel.
	if channelName != "" && s.session.AgoraChannel != "" && channelName != s.session.AgoraChannel {
		s.mu.Unlock()
		logger.Warnf("ignoring channel webhook for unexpected channel %q (expected %q)", channelName, s.session.AgoraChannel)
		return nil
	}

	// Idempotent: if already live, skip
	if s.session.State == model.StateLive {
		s.mu.Unlock()
		logger.Infof("session already live, ignoring duplicate webhook")
		return nil
	}

	if s.session.State != model.StateReady {
		state := s.session.State
		s.mu.Unlock()
		// Return nil (200 OK) so Agora health checks and out-of-order events never receive a 5xx.
		// This also handles the NCS health check case where channelName="test_webhook" and
		// the session is always idle.
		logger.Warnf("ignoring channel webhook event %d: session is in %s state (not ready)", eventType, state)
		return nil
	}

	// clientSeq deduplication: ignore replays with the same or older sequence number.
	if clientSeq > 0 && clientSeq <= s.session.LastBroadcasterClientSeq {
		s.mu.Unlock()
		logger.Infof("ignoring duplicate broadcaster-join event (clientSeq %d ≤ last %d)", clientSeq, s.session.LastBroadcasterClientSeq)
		return nil
	}
	prevClientSeq := s.session.LastBroadcasterClientSeq
	if clientSeq > 0 {
		s.session.LastBroadcasterClientSeq = clientSeq
	}

	channelName = s.session.AgoraChannel
	gcpRTMPURL := s.session.GCPInputURI
	ytRTMPURL := s.session.YouTubeRTMPURL
	broadcastID := s.session.YouTubeBroadcastID
	s.mu.Unlock()

	var (
		gcpSID    string
		ytSID     string
		gcpFailed bool
		ytFailed  bool
	)

	// Start Media Push to GCP
	if s.relay.GCPRelayEnabled {
		var err error
		gcpSID, err = s.agoraMediaPush.StartMediaPush(ctx, channelName, uid, gcpRTMPURL)
		if err != nil {
			logger.Errorf("media push to GCP failed: %v", err)
			gcpFailed = true
		}
	}

	// Start Media Push to YouTube and transition broadcast to live.
	// TransitionBroadcast is only attempted when StartMediaPush succeeds, since
	// the push must be running before YouTube will accept the "live" transition.
	if s.relay.YouTubeRelayEnabled {
		var err error
		ytSID, err = s.agoraMediaPush.StartMediaPush(ctx, channelName, uid, ytRTMPURL)
		if err != nil {
			logger.Errorf("media push to YouTube failed: %v", err)
			ytFailed = true
		} else {
			if err := s.youtube.TransitionBroadcast(ctx, broadcastID, "live"); err != nil {
				logger.Errorf("youtube transition to live failed: %v", err)
			}
		}
	}

	s.mu.Lock()
	// If any enabled relay target failed to start, roll back to ready so the next
	// broadcaster-join event (or a manual retry) can re-attempt. Resetting
	// LastBroadcasterClientSeq allows the same clientSeq to be reprocessed.
	if gcpFailed || ytFailed {
		s.session.State = model.StateReady
		s.session.LastBroadcasterClientSeq = prevClientSeq
		s.mu.Unlock()
		logger.Errorf("rolling back session to ready: media push failed (gcp_failed=%v, yt_failed=%v)", gcpFailed, ytFailed)
		return nil
	}
	s.session.State = model.StateLive
	s.session.MediaPushGCPSID = gcpSID
	s.session.MediaPushYouTubeSID = ytSID
	s.mu.Unlock()

	logger.Infof("media push started: gcp_sid=%s, yt_sid=%s", gcpSID, ytSID)
	return nil
}

// Stop tears down all resources. Each step logs errors but continues to the next
// to ensure maximum resource cleanup.
func (s *LiveService) Stop(ctx context.Context) (*model.StopResponse, error) {
	s.mu.Lock()

	if s.session.State == model.StateIdle {
		s.mu.Unlock()
		return &model.StopResponse{
			State:   model.StateIdle,
			Message: "no active session",
		}, nil
	}

	s.session.State = model.StateStopping
	sessionID := s.session.ID
	gcpSID := s.session.MediaPushGCPSID
	ytSID := s.session.MediaPushYouTubeSID
	broadcastID := s.session.YouTubeBroadcastID
	channelID := s.session.GCPChannelID
	inputID := s.session.GCPInputID
	// If allocation is still in progress, cancel it so the goroutine returns early
	// and performs its own partial-resource cleanup.
	if s.allocCancel != nil {
		logger.Infof("[stop] cancelling in-flight resource allocation for session %s", sessionID)
		s.allocCancel()
	}
	s.mu.Unlock()

	logger.Infof("[stop] beginning teardown for session %s", sessionID)

	// 1. Stop Media Push (GCP)
	if s.relay.GCPRelayEnabled && gcpSID != "" {
		logger.Infof("[stop] 1/6 stopping media push to GCP (sid=%s)", gcpSID)
		if err := s.agoraMediaPush.StopMediaPush(ctx, gcpSID); err != nil {
			logger.Errorf("[stop] 1/6 stop media push (GCP) failed: %v", err)
		} else {
			logger.Infof("[stop] 1/6 media push (GCP) stopped")
		}
	} else {
		logger.Infof("[stop] 1/6 skip — GCP media push not active")
	}

	// 2. Stop Media Push (YouTube)
	if s.relay.YouTubeRelayEnabled && ytSID != "" {
		logger.Infof("[stop] 2/6 stopping media push to YouTube (sid=%s)", ytSID)
		if err := s.agoraMediaPush.StopMediaPush(ctx, ytSID); err != nil {
			logger.Errorf("[stop] 2/6 stop media push (YouTube) failed: %v", err)
		} else {
			logger.Infof("[stop] 2/6 media push (YouTube) stopped")
		}
	} else {
		logger.Infof("[stop] 2/6 skip — YouTube media push not active")
	}

	// 3. Transition YouTube broadcast to complete
	if s.relay.YouTubeRelayEnabled && broadcastID != "" {
		logger.Infof("[stop] 3/6 transitioning YouTube broadcast %s to complete", broadcastID)
		if err := s.youtube.TransitionBroadcast(ctx, broadcastID, "complete"); err != nil {
			logger.Errorf("[stop] 3/6 youtube transition to complete failed: %v", err)
		} else {
			logger.Infof("[stop] 3/6 YouTube broadcast transitioned to complete")
		}
	} else {
		logger.Infof("[stop] 3/6 skip — YouTube broadcast not active")
	}

	// 4. Stop GCP channel
	if s.relay.GCPRelayEnabled && channelID != "" {
		logger.Infof("[stop] 4/6 stopping GCP channel %s", channelID)
		if err := s.gcp.StopChannel(ctx, channelID); err != nil {
			logger.Errorf("[stop] 4/6 stop GCP channel failed: %v", err)
		} else {
			logger.Infof("[stop] 4/6 GCP channel stopped")
		}
	} else {
		logger.Infof("[stop] 4/6 skip — GCP channel not active")
	}

	// 5. Delete GCP channel
	if s.relay.GCPRelayEnabled && channelID != "" {
		logger.Infof("[stop] 5/6 deleting GCP channel %s", channelID)
		if err := s.gcp.DeleteChannel(ctx, channelID); err != nil {
			logger.Errorf("[stop] 5/6 delete GCP channel failed: %v", err)
		} else {
			logger.Infof("[stop] 5/6 GCP channel deleted")
		}
	} else {
		logger.Infof("[stop] 5/6 skip — GCP channel not active")
	}

	// 6. Delete GCP input
	if s.relay.GCPRelayEnabled && inputID != "" {
		logger.Infof("[stop] 6/6 deleting GCP input %s", inputID)
		if err := s.gcp.DeleteInput(ctx, inputID); err != nil {
			logger.Errorf("[stop] 6/6 delete GCP input failed: %v", err)
		} else {
			logger.Infof("[stop] 6/6 GCP input deleted")
		}
	} else {
		logger.Infof("[stop] 6/6 skip — GCP input not active")
	}

	// Reset session
	s.mu.Lock()
	s.session = newIdleSession()
	s.mu.Unlock()

	logger.Infof("[stop] session %s teardown complete", sessionID)

	return &model.StopResponse{
		SessionID: sessionID,
		State:     model.StateIdle,
		Message:   "session stopped, all resources cleaned up",
	}, nil
}

// setupGCPResources creates a GCP input and channel, starts the channel, and waits
// until it reaches AWAITING_INPUT state. Returns the RTMP input URI on success.
// Called in a goroutine by allocateResources.
func (s *LiveService) setupGCPResources(ctx context.Context, inputID, channelID string) (string, error) {
	logger.Infof("[GCP] creating input %s", inputID)
	_, uri, err := s.gcp.CreateInput(ctx, inputID)
	if err != nil {
		return "", fmt.Errorf("gcp create input: %w", err)
	}
	logger.Infof("[GCP] input ready, rtmp uri: %s", uri)

	logger.Infof("[GCP] creating channel %s", channelID)
	if _, err = s.gcp.CreateChannel(ctx, channelID, inputID); err != nil {
		return "", fmt.Errorf("gcp create channel: %w", err)
	}
	logger.Infof("[GCP] channel created, starting...")

	// StartChannel must be called before WaitForChannelReady.
	// After CreateChannel the channel is in STOPPED state; only after
	// StartChannel does it transition STARTING → AWAITING_INPUT.
	if err = s.gcp.StartChannel(ctx, channelID); err != nil {
		return "", fmt.Errorf("gcp start channel: %w", err)
	}
	logger.Infof("[GCP] channel started, waiting for AWAITING_INPUT...")

	if err = s.gcp.WaitForChannelReady(ctx, channelID); err != nil {
		return "", fmt.Errorf("gcp wait for channel: %w", err)
	}
	logger.Infof("[GCP] channel ready")
	return uri, nil
}

// setupYouTubeResources creates a YouTube broadcast and stream, then binds them.
// Returns (broadcastID, streamID, rtmpURL, streamKey) on success.
// Called in a goroutine by allocateResources.
func (s *LiveService) setupYouTubeResources(ctx context.Context, sessionID string) (string, string, string, string, error) {
	logger.Infof("[YouTube] creating broadcast")
	broadcastID, err := s.youtube.CreateBroadcast(ctx, fmt.Sprintf("DeltaCast Live %s", sessionID))
	if err != nil {
		return "", "", "", "", fmt.Errorf("youtube create broadcast: %w", err)
	}
	logger.Infof("[YouTube] broadcast created: %s", broadcastID)

	logger.Infof("[YouTube] creating stream")
	streamID, rtmpURL, streamKey, err := s.youtube.CreateStream(ctx)
	if err != nil {
		return "", "", "", "", fmt.Errorf("youtube create stream: %w", err)
	}
	logger.Infof("[YouTube] stream created: %s", streamID)

	logger.Infof("[YouTube] binding broadcast to stream")
	if err = s.youtube.BindBroadcastToStream(ctx, broadcastID, streamID); err != nil {
		return "", "", "", "", fmt.Errorf("youtube bind: %w", err)
	}
	logger.Infof("[YouTube] broadcast bound to stream, ready")
	return broadcastID, streamID, rtmpURL, streamKey, nil
}

// Status returns the current session state and playback URLs.
func (s *LiveService) Status() *model.StatusResponse {
	s.mu.Lock()
	defer s.mu.Unlock()

	return &model.StatusResponse{
		SessionID:       s.session.ID,
		State:           s.session.State,
		GCPPlaybackURL:  s.session.GCPPlaybackURL,
		YouTubeWatchURL: s.session.YouTubeWatchURL,
	}
}
