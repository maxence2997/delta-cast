package provider

import (
	"context"
	"fmt"
	"time"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/option"
	"google.golang.org/api/youtube/v3"
)

type youtubeProvider struct {
	clientID     string
	clientSecret string
	refreshToken string
	service      *youtube.Service
}

// NewYouTubeProvider creates a new YouTubeProvider.
func NewYouTubeProvider(clientID, clientSecret, refreshToken string) YouTubeProvider {
	return &youtubeProvider{
		clientID:     clientID,
		clientSecret: clientSecret,
		refreshToken: refreshToken,
	}
}

func (p *youtubeProvider) getService(ctx context.Context) (*youtube.Service, error) {
	if p.service != nil {
		return p.service, nil
	}

	config := &oauth2.Config{
		ClientID:     p.clientID,
		ClientSecret: p.clientSecret,
		Endpoint:     google.Endpoint,
		Scopes:       []string{youtube.YoutubeForceSslScope},
	}

	token := &oauth2.Token{
		RefreshToken: p.refreshToken,
	}

	tokenSource := config.TokenSource(context.Background(), token)
	svc, err := youtube.NewService(context.Background(), option.WithTokenSource(tokenSource))
	if err != nil {
		return nil, fmt.Errorf("create youtube service: %w", err)
	}

	p.service = svc
	return svc, nil
}

// CreateBroadcast creates a YouTube live broadcast and returns the broadcast ID.
func (p *youtubeProvider) CreateBroadcast(ctx context.Context, title string) (string, error) {
	svc, err := p.getService(ctx)
	if err != nil {
		return "", err
	}

	broadcast := &youtube.LiveBroadcast{
		Snippet: &youtube.LiveBroadcastSnippet{
			Title: title,
			// ScheduledStartTime is required by the YouTube API.
			// Set to now so the broadcast is immediately ready to go live.
			ScheduledStartTime: time.Now().UTC().Format(time.RFC3339),
		},
		ContentDetails: &youtube.LiveBroadcastContentDetails{
			EnableAutoStart:   true,
			EnableAutoStop:    true,
			LatencyPreference: "ultraLow",
		},
		Status: &youtube.LiveBroadcastStatus{
			PrivacyStatus: "unlisted",
		},
	}

	resp, err := svc.LiveBroadcasts.Insert([]string{"snippet", "contentDetails", "status"}, broadcast).Context(ctx).Do()
	if err != nil {
		return "", fmt.Errorf("create broadcast: %w", err)
	}

	return resp.Id, nil
}

// CreateStream creates a YouTube live stream and returns (streamID, rtmpURL, streamKey).
func (p *youtubeProvider) CreateStream(ctx context.Context) (string, string, string, error) {
	svc, err := p.getService(ctx)
	if err != nil {
		return "", "", "", err
	}

	stream := &youtube.LiveStream{
		Snippet: &youtube.LiveStreamSnippet{
			Title: "DeltaCast Stream",
		},
		Cdn: &youtube.CdnSettings{
			FrameRate:     "30fps",
			IngestionType: "rtmp",
			Resolution:    "720p",
		},
	}

	resp, err := svc.LiveStreams.Insert([]string{"snippet", "cdn"}, stream).Context(ctx).Do()
	if err != nil {
		return "", "", "", fmt.Errorf("create stream: %w", err)
	}

	rtmpURL := resp.Cdn.IngestionInfo.IngestionAddress
	streamKey := resp.Cdn.IngestionInfo.StreamName

	return resp.Id, rtmpURL, streamKey, nil
}

// BindBroadcastToStream binds a broadcast to a stream.
func (p *youtubeProvider) BindBroadcastToStream(ctx context.Context, broadcastID string, streamID string) error {
	svc, err := p.getService(ctx)
	if err != nil {
		return err
	}

	_, err = svc.LiveBroadcasts.Bind(broadcastID, []string{"id", "contentDetails"}).StreamId(streamID).Context(ctx).Do()
	if err != nil {
		return fmt.Errorf("bind broadcast to stream: %w", err)
	}

	return nil
}

// TransitionBroadcast transitions a broadcast to the given status (testing, live, complete).
func (p *youtubeProvider) TransitionBroadcast(ctx context.Context, broadcastID string, status string) error {
	svc, err := p.getService(ctx)
	if err != nil {
		return err
	}

	_, err = svc.LiveBroadcasts.Transition(status, broadcastID, []string{"id", "status"}).Context(ctx).Do()
	if err != nil {
		return fmt.Errorf("transition broadcast to %s: %w", status, err)
	}

	return nil
}

// GetWatchURL returns the YouTube watch URL for a broadcast.
func (p *youtubeProvider) GetWatchURL(broadcastID string) string {
	return fmt.Sprintf("https://www.youtube.com/watch?v=%s", broadcastID)
}
