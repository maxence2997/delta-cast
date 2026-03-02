package service

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/maxence2997/delta-cast/server/internal/model"
	"github.com/maxence2997/delta-cast/server/internal/provider"
)

// --- Mock Providers ---

type mockAgoraToken struct{}

func (m *mockAgoraToken) GenerateRTCToken(channelName string, uid uint32, ttl uint32) (string, error) {
	return "mock-token-" + channelName, nil
}

type mockAgoraMediaPush struct {
	startCount int
	stopCount  atomic.Int32
	listCount  atomic.Int32
	startErr   error
	listResult []provider.ConverterInfo
	listErr    error
}

func (m *mockAgoraMediaPush) StartMediaPush(ctx context.Context, name string, channelName string, uid uint32, rtmpURL string) (string, error) {
	m.startCount++
	if m.startErr != nil {
		return "", m.startErr
	}
	return "mock-sid-" + name, nil
}

func (m *mockAgoraMediaPush) StopMediaPush(ctx context.Context, converterID string) error {
	m.stopCount.Add(1)
	return nil
}

func (m *mockAgoraMediaPush) ListConvertersByChannel(ctx context.Context, channelName string) ([]provider.ConverterInfo, error) {
	m.listCount.Add(1)
	return m.listResult, m.listErr
}

type mockGCP struct {
	createInputCalled   bool
	createChannelCalled bool
	startChannelCalled  bool
	stopChannelCalled   bool
	deleteChannelCalled bool
	deleteInputCalled   bool
}

func (m *mockGCP) CreateInput(ctx context.Context, inputID string) (string, string, error) {
	m.createInputCalled = true
	return inputID, "rtmp://gcp-input/" + inputID, nil
}

func (m *mockGCP) CreateChannel(ctx context.Context, channelID string, inputID string) (string, error) {
	m.createChannelCalled = true
	return channelID, nil
}

func (m *mockGCP) StartChannel(ctx context.Context, channelID string) error {
	m.startChannelCalled = true
	return nil
}

func (m *mockGCP) StopChannel(ctx context.Context, channelID string) error {
	m.stopChannelCalled = true
	return nil
}

func (m *mockGCP) DeleteChannel(ctx context.Context, channelID string) error {
	m.deleteChannelCalled = true
	return nil
}

func (m *mockGCP) DeleteInput(ctx context.Context, inputID string) error {
	m.deleteInputCalled = true
	return nil
}

func (m *mockGCP) GetPlaybackURL(channelID string) string {
	return "https://cdn.example.com/" + channelID + "/main.m3u8"
}

func (m *mockGCP) WaitForChannelReady(ctx context.Context, channelID string) error {
	return nil
}

type mockYouTube struct {
	createBroadcastCalled bool
	createStreamCalled    bool
	bindCalled            bool
	transitionCalls       []string
}

func (m *mockYouTube) CreateBroadcast(ctx context.Context, title string) (string, error) {
	m.createBroadcastCalled = true
	return "broadcast-123", nil
}

func (m *mockYouTube) CreateStream(ctx context.Context) (string, string, string, error) {
	m.createStreamCalled = true
	return "stream-456", "rtmp://yt-ingest", "stream-key-789", nil
}

func (m *mockYouTube) BindBroadcastToStream(ctx context.Context, broadcastID string, streamID string) error {
	m.bindCalled = true
	return nil
}

func (m *mockYouTube) TransitionBroadcast(ctx context.Context, broadcastID string, status string) error {
	m.transitionCalls = append(m.transitionCalls, status)
	return nil
}

func (m *mockYouTube) GetWatchURL(broadcastID string) string {
	return "https://youtube.com/watch?v=" + broadcastID
}

// --- Helper ---

func newTestService() (*LiveService, *mockAgoraMediaPush, *mockGCP, *mockYouTube) {
	tokenProv := &mockAgoraToken{}
	pushProv := &mockAgoraMediaPush{}
	gcpProv := &mockGCP{}
	ytProv := &mockYouTube{}

	svc := NewLiveService(tokenProv, pushProv, gcpProv, ytProv, RelayOptions{GCPRelayEnabled: true, YouTubeRelayEnabled: true})
	return svc, pushProv, gcpProv, ytProv
}

// --- Tests ---

func TestPrepare_FromIdle(t *testing.T) {
	svc, _, _, _ := newTestService()

	resp, err := svc.Prepare(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resp.State != model.StatePreparing {
		t.Errorf("expected state preparing, got %s", resp.State)
	}
	if resp.SessionID == "" {
		t.Error("expected non-empty session ID")
	}
}

func TestPrepare_AlreadyPreparing(t *testing.T) {
	svc, _, _, _ := newTestService()

	resp1, _ := svc.Prepare(context.Background())
	resp2, _ := svc.Prepare(context.Background())

	if resp2.SessionID != resp1.SessionID {
		t.Errorf("expected same session ID, got %s and %s", resp1.SessionID, resp2.SessionID)
	}
	if resp2.Message != "session already exists" {
		t.Errorf("expected 'session already exists', got %s", resp2.Message)
	}
}

func TestStart_NotReady(t *testing.T) {
	svc, _, _, _ := newTestService()

	_, err := svc.Start(context.Background(), "app-id")
	if err == nil {
		t.Error("expected error when starting without prepare")
	}
}

func TestStart_WhenReady(t *testing.T) {
	svc, _, _, _ := newTestService()

	svc.session.ID = "test-session"
	svc.session.State = model.StateReady
	svc.session.AgoraChannel = "deltacast-test"

	resp, err := svc.Start(context.Background(), "test-app-id")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resp.AgoraToken == "" {
		t.Error("expected non-empty token")
	}
	if resp.AgoraChannel != "deltacast-test" {
		t.Errorf("expected channel deltacast-test, got %s", resp.AgoraChannel)
	}
}

func TestHandleChannelWebhook_BroadcasterJoin_MovesToLive(t *testing.T) {
	svc, pushProv, _, _ := newTestService()

	svc.session.ID = "test-session"
	svc.session.State = model.StateReady
	svc.session.AgoraChannel = "ch"
	svc.session.GCPInputURI = "rtmp://gcp"
	svc.session.YouTubeRTMPURL = "rtmp://yt"
	svc.session.YouTubeBroadcastID = "bc-123"

	err := svc.HandleChannelWebhook(context.Background(), "", 103, 12345, "ch", 1000)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if svc.session.State != model.StateLive {
		t.Errorf("expected state live, got %s", svc.session.State)
	}
	if pushProv.startCount != 2 {
		t.Errorf("expected 2 media push starts, got %d", pushProv.startCount)
	}
}

func TestHandleChannelWebhook_Idempotent(t *testing.T) {
	svc, pushProv, _, _ := newTestService()

	svc.session.ID = "test-session"
	svc.session.State = model.StateLive
	svc.session.AgoraChannel = "ch"

	err := svc.HandleChannelWebhook(context.Background(), "", 103, 12345, "ch", 1000)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if pushProv.startCount != 0 {
		t.Errorf("expected 0 media push starts for duplicate webhook, got %d", pushProv.startCount)
	}
}

func TestHandleChannelWebhook_DuplicateClientSeq(t *testing.T) {
	svc, pushProv, _, _ := newTestService()

	svc.session.ID = "test-session"
	svc.session.State = model.StateReady
	svc.session.AgoraChannel = "ch"
	svc.session.GCPInputURI = "rtmp://gcp"
	svc.session.YouTubeRTMPURL = "rtmp://yt"
	svc.session.LastBroadcasterClientSeq = 1000

	// Replayed event with same clientSeq — should be ignored even in StateReady.
	err := svc.HandleChannelWebhook(context.Background(), "", 103, 12345, "ch", 1000)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if pushProv.startCount != 0 {
		t.Errorf("expected 0 media push starts for replayed clientSeq, got %d", pushProv.startCount)
	}
}

func TestHandleChannelWebhook_WrongChannel(t *testing.T) {
	svc, pushProv, _, _ := newTestService()

	svc.session.ID = "test-session"
	svc.session.State = model.StateReady
	svc.session.AgoraChannel = "ch"
	svc.session.GCPInputURI = "rtmp://gcp"
	svc.session.YouTubeRTMPURL = "rtmp://yt"

	// Event from a different channel — should be silently ignored.
	err := svc.HandleChannelWebhook(context.Background(), "", 103, 12345, "other-channel", 2000)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if pushProv.startCount != 0 {
		t.Errorf("expected 0 media push starts for wrong channel, got %d", pushProv.startCount)
	}
}

func TestHandleWebhook_IgnoresOtherEvents(t *testing.T) {
	svc, pushProv, _, _ := newTestService()

	svc.session.State = model.StateReady

	err := svc.HandleChannelWebhook(context.Background(), "", 102, 0, "", 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if pushProv.startCount != 0 {
		t.Errorf("expected 0 media push starts for event 102, got %d", pushProv.startCount)
	}
}

func TestStop_CleansUpAllResources(t *testing.T) {
	svc, pushProv, gcpProv, ytProv := newTestService()

	svc.session.ID = "test-session"
	svc.session.State = model.StateLive
	svc.session.MediaPushGCPSID = "gcp-sid"
	svc.session.MediaPushYouTubeSID = "yt-sid"
	svc.session.YouTubeBroadcastID = "bc-123"
	svc.session.GCPChannelID = "channel-test"
	svc.session.GCPInputID = "input-test"

	resp, err := svc.Stop(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resp.State != model.StateIdle {
		t.Errorf("expected state idle after stop, got %s", resp.State)
	}
	if pushProv.stopCount.Load() != 2 {
		t.Errorf("expected 2 media push stops, got %d", pushProv.stopCount.Load())
	}
	if !gcpProv.stopChannelCalled {
		t.Error("expected GCP StopChannel to be called")
	}
	if !gcpProv.deleteChannelCalled {
		t.Error("expected GCP DeleteChannel to be called")
	}
	if !gcpProv.deleteInputCalled {
		t.Error("expected GCP DeleteInput to be called")
	}
	if len(ytProv.transitionCalls) != 1 || ytProv.transitionCalls[0] != "complete" {
		t.Errorf("expected youtube transition to complete, got %v", ytProv.transitionCalls)
	}
}

func TestStop_IdleSession(t *testing.T) {
	svc, _, _, _ := newTestService()

	resp, err := svc.Stop(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resp.Message != "no active session" {
		t.Errorf("expected 'no active session', got %s", resp.Message)
	}
}

func TestStatus(t *testing.T) {
	svc, _, _, _ := newTestService()

	svc.session.ID = "test-session"
	svc.session.State = model.StateLive
	svc.session.GCPPlaybackURL = "https://cdn/hls"
	svc.session.YouTubeWatchURL = "https://youtube.com/watch"

	status := svc.Status()

	if status.State != model.StateLive {
		t.Errorf("expected live, got %s", status.State)
	}
	if status.GCPPlaybackURL != "https://cdn/hls" {
		t.Errorf("unexpected GCP URL: %s", status.GCPPlaybackURL)
	}
}

func TestHandleChannelWebhook_MediaPushFails_RollsBackToReady(t *testing.T) {
	svc, pushProv, _, _ := newTestService()
	pushProv.startErr = errors.New("push failed")

	svc.session.ID = "test-session"
	svc.session.State = model.StateReady
	svc.session.AgoraChannel = "ch"
	svc.session.GCPInputURI = "rtmp://gcp"
	svc.session.YouTubeRTMPURL = "rtmp://yt"
	svc.session.YouTubeBroadcastID = "bc-123"

	err := svc.HandleChannelWebhook(context.Background(), "", 103, 12345, "ch", 500)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if svc.session.State != model.StateReady {
		t.Errorf("expected session to roll back to ready, got %s", svc.session.State)
	}
	// clientSeq must be reset so the same event can be retried.
	if svc.session.LastBroadcasterClientSeq != 0 {
		t.Errorf("expected LastBroadcasterClientSeq to be reset to 0, got %d", svc.session.LastBroadcasterClientSeq)
	}
}

func TestHandleChannelWebhook_DuplicateNoticeID(t *testing.T) {
	svc, pushProv, _, _ := newTestService()

	svc.session.ID = "test-session"
	svc.session.State = model.StateReady
	svc.session.AgoraChannel = "ch"
	svc.session.GCPInputURI = "rtmp://gcp"
	svc.session.YouTubeRTMPURL = "rtmp://yt"
	svc.session.YouTubeBroadcastID = "bc-123"

	// First delivery — should succeed and trigger media push.
	if err := svc.HandleChannelWebhook(context.Background(), "notice-abc", 103, 12345, "ch", 1000); err != nil {
		t.Fatalf("first delivery: unexpected error: %v", err)
	}
	if pushProv.startCount != 2 {
		t.Fatalf("expected 2 starts after first delivery, got %d", pushProv.startCount)
	}

	// Second delivery with same noticeId — must be ignored entirely.
	if err := svc.HandleChannelWebhook(context.Background(), "notice-abc", 103, 12345, "ch", 1000); err != nil {
		t.Fatalf("second delivery: unexpected error: %v", err)
	}
	if pushProv.startCount != 2 {
		t.Errorf("expected no additional starts for duplicate noticeId, got %d total", pushProv.startCount)
	}
}

func TestHandleMediaPushWebhook_DuplicateNoticeID(t *testing.T) {
	svc, _, _, _ := newTestService()

	// First delivery — should process normally (log only, no state change).
	if err := svc.HandleMediaPushWebhook(context.Background(), "notice-xyz", 1, "conv-1", "", ""); err != nil {
		t.Fatalf("first delivery: unexpected error: %v", err)
	}

	// Second delivery with the same noticeId must be silently ignored.
	// We verify this indirectly: no panic, no error, and the noticeId stays recorded.
	if err := svc.HandleMediaPushWebhook(context.Background(), "notice-xyz", 1, "conv-1", "", ""); err != nil {
		t.Fatalf("second delivery: unexpected error: %v", err)
	}

	if _, seen := svc.session.SeenNoticeIDs["notice-xyz"]; !seen {
		t.Error("expected noticeId to remain in SeenNoticeIDs after dedup")
	}
}

// TestHandleChannelWebhook_Event103DuringPreparing_ProcessedAfterReady verifies Bug 1 fix:
// a noticeId received while the session is "preparing" must NOT be tombstoned.
// Agora's subsequent retry, arriving after the session becomes "ready", must be processed.
func TestHandleChannelWebhook_Event103DuringPreparing_ProcessedAfterReady(t *testing.T) {
	svc, pushProv, _, _ := newTestService()

	svc.session.ID = "test-session"
	svc.session.AgoraChannel = "ch"
	svc.session.GCPInputURI = "rtmp://gcp"
	svc.session.YouTubeRTMPURL = "rtmp://yt"
	svc.session.YouTubeBroadcastID = "bc-123"

	// Event 103 arrives while session is still "preparing" — must be dropped but
	// the noticeId must NOT be recorded in SeenNoticeIDs.
	svc.session.State = model.StatePreparing
	if err := svc.HandleChannelWebhook(context.Background(), "notice-early-103", 103, 999, "ch", 1000); err != nil {
		t.Fatalf("preparing delivery: unexpected error: %v", err)
	}
	if pushProv.startCount != 0 {
		t.Fatalf("expected 0 push starts during preparing, got %d", pushProv.startCount)
	}
	svc.mu.Lock()
	_, tombstoned := svc.session.SeenNoticeIDs["notice-early-103"]
	svc.mu.Unlock()
	if tombstoned {
		t.Fatal("noticeId must NOT be tombstoned when event is dropped due to preparing state")
	}

	// Session transitions to ready; Agora retries the same noticeId — must succeed.
	svc.session.State = model.StateReady
	if err := svc.HandleChannelWebhook(context.Background(), "notice-early-103", 103, 999, "ch", 1000); err != nil {
		t.Fatalf("ready delivery: unexpected error: %v", err)
	}
	if svc.session.State != model.StateLive {
		t.Errorf("expected state live after retry, got %s", svc.session.State)
	}
	if pushProv.startCount != 2 {
		t.Errorf("expected 2 push starts after retry, got %d", pushProv.startCount)
	}
}

// TestHandleChannelWebhook_Rollback_StopsSuccessfulConverter verifies Bug 2 fix:
// when GCP push succeeds but YouTube push fails, the rollback must stop the GCP converter.
func TestHandleChannelWebhook_Rollback_StopsSuccessfulConverter(t *testing.T) {
	svc, _, _, _ := newTestService()

	failSecond := &mockAgoraMediaPushFailSecond{}
	svc.agoraMediaPush = failSecond

	svc.session.ID = "test-session"
	svc.session.State = model.StateReady
	svc.session.AgoraChannel = "ch"
	svc.session.GCPInputURI = "rtmp://gcp"
	svc.session.YouTubeRTMPURL = "rtmp://yt"
	svc.session.YouTubeBroadcastID = "bc-123"

	if err := svc.HandleChannelWebhook(context.Background(), "", 103, 12345, "ch", 2000); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if svc.session.State != model.StateReady {
		t.Errorf("expected rollback to ready, got %s", svc.session.State)
	}

	// Wait briefly for the rollback goroutine to run.
	time.Sleep(50 * time.Millisecond)

	if failSecond.stopCount.Load() == 0 {
		t.Error("expected StopMediaPush to be called for the successfully created GCP converter")
	}
}

// mockAgoraMediaPushFailSecond succeeds on the first StartMediaPush call and fails on the second.
type mockAgoraMediaPushFailSecond struct {
	callCount int
	stopCount atomic.Int32
	listCount int
}

func (m *mockAgoraMediaPushFailSecond) StartMediaPush(_ context.Context, name, _ string, _ uint32, _ string) (string, error) {
	m.callCount++
	if m.callCount == 1 {
		return "gcp-converter-sid", nil
	}
	return "", errors.New("push failed")
}

func (m *mockAgoraMediaPushFailSecond) StopMediaPush(_ context.Context, _ string) error {
	m.stopCount.Add(1)
	return nil
}

func (m *mockAgoraMediaPushFailSecond) ListConvertersByChannel(_ context.Context, _ string) ([]provider.ConverterInfo, error) {
	m.listCount++
	return nil, nil
}

// TestHandleChannelWebhook_Rollback_ListsOrphanedConverters verifies Bug 3 fix:
// when both pushes fail with no SID (e.g. 409), rollback must call ListConvertersByChannel
// and stop all returned converters.
func TestHandleChannelWebhook_Rollback_ListsOrphanedConverters(t *testing.T) {
	svc, _, _, _ := newTestService()

	orphanPush := &mockAgoraMediaPush{
		startErr: errors.New("409 conflict"),
		listResult: []provider.ConverterInfo{
			{ID: "orphan-id-1", Name: "ch_gcp"},
			{ID: "orphan-id-2", Name: "ch_yt"},
		},
	}
	svc.agoraMediaPush = orphanPush

	svc.session.ID = "test-session"
	svc.session.State = model.StateReady
	svc.session.AgoraChannel = "ch"
	svc.session.GCPInputURI = "rtmp://gcp"
	svc.session.YouTubeRTMPURL = "rtmp://yt"
	svc.session.YouTubeBroadcastID = "bc-123"

	if err := svc.HandleChannelWebhook(context.Background(), "", 103, 12345, "ch", 3000); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if svc.session.State != model.StateReady {
		t.Errorf("expected rollback to ready, got %s", svc.session.State)
	}

	// Wait briefly for the rollback goroutine to run.
	time.Sleep(50 * time.Millisecond)

	if orphanPush.listCount.Load() == 0 {
		t.Error("expected ListConvertersByChannel to be called for orphan cleanup")
	}
	// Both orphaned converters should have been stopped.
	if orphanPush.stopCount.Load() != 2 {
		t.Errorf("expected 2 StopMediaPush calls for orphaned converters, got %d", orphanPush.stopCount.Load())
	}
}

// TestHandleMediaPushWebhook_ConverterFailed_ResetsToReady verifies that a converter
// "failed" state event (eventType=3) while live resets the session back to ready so
// that the next broadcaster-join (eventType=103) can restart Media Push.
func TestHandleMediaPushWebhook_ConverterFailed_ResetsToReady(t *testing.T) {
	svc, _, _, _ := newTestService()

	svc.session.State = model.StateLive
	svc.session.AgoraChannel = "ch"
	svc.session.GCPInputURI = "rtmp://gcp"
	svc.session.YouTubeRTMPURL = "rtmp://yt"
	svc.session.MediaPushGCPSID = "gcp-sid-active"
	svc.session.MediaPushYouTubeSID = "yt-sid-active"
	svc.session.LastBroadcasterClientSeq = 1000

	if err := svc.HandleMediaPushWebhook(context.Background(), "", 3, "gcp-sid-active", "failed", ""); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if svc.session.State != model.StateReady {
		t.Errorf("expected state ready after converter failure, got %s", svc.session.State)
	}
	// Failed SID must be cleared so Stop() does not try to stop it again.
	if svc.session.MediaPushGCPSID != "" {
		t.Errorf("expected GCP SID to be cleared, got %q", svc.session.MediaPushGCPSID)
	}
	// The other SID must be preserved.
	if svc.session.MediaPushYouTubeSID != "yt-sid-active" {
		t.Errorf("expected YouTube SID to be preserved, got %q", svc.session.MediaPushYouTubeSID)
	}
	// clientSeq must be reset so the same broadcaster-join can be reprocessed.
	if svc.session.LastBroadcasterClientSeq != 0 {
		t.Errorf("expected LastBroadcasterClientSeq reset to 0, got %d", svc.session.LastBroadcasterClientSeq)
	}
}

// TestHandleMediaPushWebhook_ConverterDestroyedInternalError_ResetsToReady verifies that
// an unexpected converter destruction (eventType=4, reason="Internal Error") while live
// resets the session back to ready.
func TestHandleMediaPushWebhook_ConverterDestroyedInternalError_ResetsToReady(t *testing.T) {
	svc, _, _, _ := newTestService()

	svc.session.State = model.StateLive
	svc.session.AgoraChannel = "ch"
	svc.session.MediaPushGCPSID = "gcp-sid-active"
	svc.session.MediaPushYouTubeSID = "yt-sid-active"
	svc.session.LastBroadcasterClientSeq = 500

	if err := svc.HandleMediaPushWebhook(context.Background(), "", 4, "yt-sid-active", "", "Internal Error"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if svc.session.State != model.StateReady {
		t.Errorf("expected state ready after unexpected destroy, got %s", svc.session.State)
	}
	if svc.session.MediaPushYouTubeSID != "" {
		t.Errorf("expected YouTube SID to be cleared, got %q", svc.session.MediaPushYouTubeSID)
	}
	if svc.session.MediaPushGCPSID != "gcp-sid-active" {
		t.Errorf("expected GCP SID to be preserved, got %q", svc.session.MediaPushGCPSID)
	}
	if svc.session.LastBroadcasterClientSeq != 0 {
		t.Errorf("expected LastBroadcasterClientSeq reset to 0, got %d", svc.session.LastBroadcasterClientSeq)
	}
}

// TestHandleMediaPushWebhook_ConverterDestroyed_NormalStop_NoReset verifies that a
// normal converter destroy (reason != "Internal Error") does NOT reset the session.
func TestHandleMediaPushWebhook_ConverterDestroyed_NormalStop_NoReset(t *testing.T) {
	svc, _, _, _ := newTestService()

	svc.session.State = model.StateLive

	if err := svc.HandleMediaPushWebhook(context.Background(), "", 4, "some-sid", "", "stop"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if svc.session.State != model.StateLive {
		t.Errorf("expected state to remain live on normal stop, got %s", svc.session.State)
	}
}

// TestHandleMediaPushWebhook_ConverterFailed_Then103_RestartsMediaPush is an end-to-end
// test verifying the full recovery flow: converter fails → session resets to ready →
// next broadcaster-join (eventType=103) restarts Media Push.
func TestHandleMediaPushWebhook_ConverterFailed_Then103_RestartsMediaPush(t *testing.T) {
	svc, pushProv, _, _ := newTestService()

	svc.session.ID = "test-session"
	svc.session.State = model.StateLive
	svc.session.AgoraChannel = "ch"
	svc.session.GCPInputURI = "rtmp://gcp"
	svc.session.YouTubeRTMPURL = "rtmp://yt"
	svc.session.MediaPushGCPSID = "gcp-sid-old"
	svc.session.MediaPushYouTubeSID = "yt-sid-old"
	svc.session.LastBroadcasterClientSeq = 1000

	// Step 1: GCP converter fails — session must drop to ready.
	if err := svc.HandleMediaPushWebhook(context.Background(), "mp-fail", 3, "gcp-sid-old", "failed", ""); err != nil {
		t.Fatalf("media push webhook: unexpected error: %v", err)
	}
	if svc.session.State != model.StateReady {
		t.Fatalf("expected ready after converter failure, got %s", svc.session.State)
	}

	// Step 2: broadcaster rejoins (same clientSeq — must be accepted after reset).
	if err := svc.HandleChannelWebhook(context.Background(), "103-retry", 103, 12345, "ch", 1000); err != nil {
		t.Fatalf("channel webhook: unexpected error: %v", err)
	}
	if svc.session.State != model.StateLive {
		t.Errorf("expected live after Media Push restart, got %s", svc.session.State)
	}
	if pushProv.startCount != 2 {
		t.Errorf("expected 2 StartMediaPush calls on restart, got %d", pushProv.startCount)
	}
}
