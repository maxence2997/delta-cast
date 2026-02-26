package service

import (
	"context"
	"testing"

	"github.com/maxence2997/delta-cast/server/internal/model"
)

// --- Mock Providers ---

type mockAgoraToken struct{}

func (m *mockAgoraToken) GenerateRTCToken(channelName string, uid uint32, ttl uint32) (string, error) {
	return "mock-token-" + channelName, nil
}

type mockAgoraMediaPush struct {
	startCount int
	stopCount  int
}

func (m *mockAgoraMediaPush) StartMediaPush(ctx context.Context, channelName string, uid uint32, rtmpURL string) (string, error) {
	m.startCount++
	return "mock-sid", nil
}

func (m *mockAgoraMediaPush) StopMediaPush(ctx context.Context, converterID string) error {
	m.stopCount++
	return nil
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

	svc := NewLiveService(tokenProv, pushProv, gcpProv, ytProv)
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

func TestHandleWebhook_Event101_MovesToLive(t *testing.T) {
	svc, pushProv, _, _ := newTestService()

	svc.session.ID = "test-session"
	svc.session.State = model.StateReady
	svc.session.AgoraChannel = "ch"
	svc.session.GCPInputURI = "rtmp://gcp"
	svc.session.YouTubeRTMPURL = "rtmp://yt"
	svc.session.YouTubeBroadcastID = "bc-123"

	err := svc.HandleAgoraWebhook(context.Background(), 101, 12345)
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

func TestHandleWebhook_Idempotent(t *testing.T) {
	svc, pushProv, _, _ := newTestService()

	svc.session.ID = "test-session"
	svc.session.State = model.StateLive
	svc.session.AgoraChannel = "ch"

	err := svc.HandleAgoraWebhook(context.Background(), 101, 12345)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if pushProv.startCount != 0 {
		t.Errorf("expected 0 media push starts for duplicate webhook, got %d", pushProv.startCount)
	}
}

func TestHandleWebhook_IgnoresOtherEvents(t *testing.T) {
	svc, pushProv, _, _ := newTestService()

	svc.session.State = model.StateReady

	err := svc.HandleAgoraWebhook(context.Background(), 102, 0)
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
	if pushProv.stopCount != 2 {
		t.Errorf("expected 2 media push stops, got %d", pushProv.stopCount)
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
