// Package service contains the core business logic for live streaming orchestration.
package service

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/maxence2997/delta-cast/server/internal/model"
	"github.com/maxence2997/delta-cast/server/internal/provider"
)

// LiveService orchestrates the live streaming session lifecycle.
type LiveService struct {
	mu sync.Mutex

	session *model.Session

	agoraToken     provider.AgoraTokenProvider
	agoraMediaPush provider.AgoraMediaPushProvider
	gcp            provider.GCPLiveStreamProvider
	youtube        provider.YouTubeProvider
}

// NewLiveService creates a new LiveService with the provided providers.
func NewLiveService(
	agoraToken provider.AgoraTokenProvider,
	agoraMediaPush provider.AgoraMediaPushProvider,
	gcp provider.GCPLiveStreamProvider,
	youtube provider.YouTubeProvider,
) *LiveService {
	return &LiveService{
		session: &model.Session{
			State: model.StateIdle,
		},
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
		ID:           sessionID,
		State:        model.StatePreparing,
		CreatedAt:    time.Now(),
		AgoraChannel: fmt.Sprintf("deltacast-%s", sessionID),
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
func (s *LiveService) allocateResources(sessionID string) {
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	inputID := fmt.Sprintf("input-%s", sessionID)
	channelID := fmt.Sprintf("channel-%s", sessionID)

	var (
		gcpErr error
		ytErr  error
		wg     sync.WaitGroup
	)

	// GCP: Create Input → Create Channel → Wait for ready → Start Channel
	var gcpInputURI string
	wg.Add(1)
	go func() {
		defer wg.Done()

		_, uri, err := s.gcp.CreateInput(ctx, inputID)
		if err != nil {
			gcpErr = fmt.Errorf("gcp create input: %w", err)
			return
		}
		gcpInputURI = uri

		_, err = s.gcp.CreateChannel(ctx, channelID, inputID)
		if err != nil {
			gcpErr = fmt.Errorf("gcp create channel: %w", err)
			return
		}

		if err := s.gcp.WaitForChannelReady(ctx, channelID); err != nil {
			gcpErr = fmt.Errorf("gcp wait for channel: %w", err)
			return
		}

		if err := s.gcp.StartChannel(ctx, channelID); err != nil {
			gcpErr = fmt.Errorf("gcp start channel: %w", err)
			return
		}
	}()

	// YouTube: Create Broadcast → Create Stream → Bind
	var (
		broadcastID string
		streamID    string
		rtmpURL     string
		streamKey   string
	)
	wg.Add(1)
	go func() {
		defer wg.Done()

		var err error
		broadcastID, err = s.youtube.CreateBroadcast(ctx, fmt.Sprintf("DeltaCast Live %s", sessionID))
		if err != nil {
			ytErr = fmt.Errorf("youtube create broadcast: %w", err)
			return
		}

		streamID, rtmpURL, streamKey, err = s.youtube.CreateStream(ctx)
		if err != nil {
			ytErr = fmt.Errorf("youtube create stream: %w", err)
			return
		}

		if err := s.youtube.BindBroadcastToStream(ctx, broadcastID, streamID); err != nil {
			ytErr = fmt.Errorf("youtube bind: %w", err)
			return
		}
	}()

	wg.Wait()

	s.mu.Lock()
	defer s.mu.Unlock()

	// Check session hasn't been reset while we were allocating
	if s.session.ID != sessionID {
		log.Printf("session changed during allocation, discarding results for %s", sessionID)
		return
	}

	if gcpErr != nil {
		log.Printf("ERROR: resource allocation failed (GCP): %v", gcpErr)
		s.session.State = model.StateIdle
		return
	}
	if ytErr != nil {
		log.Printf("ERROR: resource allocation failed (YouTube): %v", ytErr)
		s.session.State = model.StateIdle
		return
	}

	// Update session with resource details
	s.session.GCPInputID = inputID
	s.session.GCPChannelID = channelID
	s.session.GCPInputURI = gcpInputURI
	s.session.GCPPlaybackURL = s.gcp.GetPlaybackURL(channelID)
	s.session.YouTubeBroadcastID = broadcastID
	s.session.YouTubeStreamID = streamID
	s.session.YouTubeStreamKey = streamKey
	s.session.YouTubeRTMPURL = fmt.Sprintf("%s/%s", rtmpURL, streamKey)
	s.session.YouTubeWatchURL = s.youtube.GetWatchURL(broadcastID)
	s.session.State = model.StateReady

	log.Printf("session %s resources ready", sessionID)
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

// HandleAgoraWebhook processes Agora NCS events.
// For event 101 (broadcaster joins channel), it triggers Media Push to both targets.
// uid is the broadcaster's Agora RTC UID extracted from the NCS payload.
func (s *LiveService) HandleAgoraWebhook(ctx context.Context, eventType int, uid uint32) error {
	// Only handle event 101 (channel create / broadcaster joined)
	if eventType != 101 {
		log.Printf("ignoring agora event type %d", eventType)
		return nil
	}

	s.mu.Lock()

	// Idempotent: if already live, skip
	if s.session.State == model.StateLive {
		s.mu.Unlock()
		log.Printf("session already live, ignoring duplicate webhook")
		return nil
	}

	if s.session.State != model.StateReady {
		state := s.session.State
		s.mu.Unlock()
		return fmt.Errorf("session is in %s state, expected ready for webhook", state)
	}

	s.session.State = model.StateLive
	channelName := s.session.AgoraChannel
	gcpRTMPURL := s.session.GCPInputURI
	ytRTMPURL := s.session.YouTubeRTMPURL
	broadcastID := s.session.YouTubeBroadcastID
	s.mu.Unlock()

	// Start Media Push to GCP
	gcpSID, err := s.agoraMediaPush.StartMediaPush(ctx, channelName, uid, gcpRTMPURL)
	if err != nil {
		log.Printf("ERROR: media push to GCP failed: %v", err)
	}

	// Start Media Push to YouTube
	ytSID, err := s.agoraMediaPush.StartMediaPush(ctx, channelName, uid, ytRTMPURL)
	if err != nil {
		log.Printf("ERROR: media push to YouTube failed: %v", err)
	}

	// Transition YouTube broadcast to live
	if err := s.youtube.TransitionBroadcast(ctx, broadcastID, "live"); err != nil {
		log.Printf("ERROR: youtube transition to live failed: %v", err)
	}

	s.mu.Lock()
	s.session.MediaPushGCPSID = gcpSID
	s.session.MediaPushYouTubeSID = ytSID
	s.mu.Unlock()

	log.Printf("media push started: gcp_sid=%s, yt_sid=%s", gcpSID, ytSID)
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
	s.mu.Unlock()

	// 1. Stop Media Push (GCP)
	if gcpSID != "" {
		if err := s.agoraMediaPush.StopMediaPush(ctx, gcpSID); err != nil {
			log.Printf("ERROR: stop media push (GCP) failed: %v", err)
		}
	}

	// 2. Stop Media Push (YouTube)
	if ytSID != "" {
		if err := s.agoraMediaPush.StopMediaPush(ctx, ytSID); err != nil {
			log.Printf("ERROR: stop media push (YouTube) failed: %v", err)
		}
	}

	// 3. Transition YouTube broadcast to complete
	if broadcastID != "" {
		if err := s.youtube.TransitionBroadcast(ctx, broadcastID, "complete"); err != nil {
			log.Printf("ERROR: youtube transition to complete failed: %v", err)
		}
	}

	// 4. Stop GCP channel
	if channelID != "" {
		if err := s.gcp.StopChannel(ctx, channelID); err != nil {
			log.Printf("ERROR: stop GCP channel failed: %v", err)
		}
	}

	// 5. Delete GCP channel
	if channelID != "" {
		if err := s.gcp.DeleteChannel(ctx, channelID); err != nil {
			log.Printf("ERROR: delete GCP channel failed: %v", err)
		}
	}

	// 6. Delete GCP input
	if inputID != "" {
		if err := s.gcp.DeleteInput(ctx, inputID); err != nil {
			log.Printf("ERROR: delete GCP input failed: %v", err)
		}
	}

	// Reset session
	s.mu.Lock()
	s.session = &model.Session{State: model.StateIdle}
	s.mu.Unlock()

	log.Printf("session %s stopped and cleaned up", sessionID)

	return &model.StopResponse{
		SessionID: sessionID,
		State:     model.StateIdle,
		Message:   "session stopped, all resources cleaned up",
	}, nil
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
