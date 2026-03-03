package provider

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"time"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/option"
	"google.golang.org/api/youtube/v3"
)

// YouTubeAuth is a sealed interface representing a YouTube authentication strategy.
// Use [PersonalYouTubeAuth] for individual Google accounts (OAuth2 refresh token),
// or [EnterpriseYouTubeAuth] for Google Workspace accounts (SA + Domain-Wide Delegation).
type YouTubeAuth interface {
	youtubeAuth() // sealed — only types in this package can implement
}

// PersonalYouTubeAuth authenticates using an OAuth2 refresh token obtained via
// Google's consent screen. Used for individual (non-Workspace) Google accounts.
// All three fields are required.
type PersonalYouTubeAuth struct {
	ClientID     string
	ClientSecret string
	RefreshToken string
}

func (PersonalYouTubeAuth) youtubeAuth() {}

// EnterpriseYouTubeAuth authenticates via Service Account + Domain-Wide Delegation (DWD).
// The SA impersonates the target Google Workspace user to operate their YouTube channel.
//
// Prerequisites:
//   - The SA key must be a standard SA key JSON (not a WIF external_account config).
//   - A Google Workspace org admin must grant the SA DWD access to
//     https://www.googleapis.com/auth/youtube in Google Admin Console.
type EnterpriseYouTubeAuth struct {
	// SAKeyPath is the file path to the SA private key JSON file.
	// Takes priority over SAKeyJSON when both are set.
	SAKeyPath string
	// SAKeyJSON is the full inline content of the SA private key JSON.
	// Used as fallback when file mounting is unavailable (e.g. PaaS).
	SAKeyJSON string
	// ImpersonateEmail is the Google Workspace user email whose YouTube channel
	// will be operated on behalf of. Must belong to the same Workspace org.
	ImpersonateEmail string
}

func (EnterpriseYouTubeAuth) youtubeAuth() {}

type youtubeProvider struct {
	auth    YouTubeAuth
	service *youtube.Service
}

// NewYouTubeProvider creates a new YouTubeProvider.
// Pass a [PersonalYouTubeAuth] for individual account mode,
// or an [EnterpriseYouTubeAuth] for Google Workspace + DWD mode.
func NewYouTubeProvider(auth YouTubeAuth) YouTubeProvider {
	return &youtubeProvider{auth: auth}
}

func (p *youtubeProvider) getService(ctx context.Context) (*youtube.Service, error) {
	if p.service != nil {
		return p.service, nil
	}

	var (
		svc *youtube.Service
		err error
	)

	switch a := p.auth.(type) {
	case EnterpriseYouTubeAuth:
		// Enterprise mode: SA + Domain-Wide Delegation.
		// A JWT is signed with the SA private key and the Subject field is set to
		// the target user's email, delegating their YouTube identity to this service.
		var jsonData []byte
		if a.SAKeyPath != "" {
			jsonData, err = os.ReadFile(a.SAKeyPath)
			if err != nil {
				return nil, fmt.Errorf("read sa key for youtube DWD: %w", err)
			}
		} else {
			jsonData = []byte(a.SAKeyJSON)
		}
		jwtCfg, err := google.JWTConfigFromJSON(jsonData, youtube.YoutubeForceSslScope)
		if err != nil {
			return nil, fmt.Errorf("parse sa key for youtube DWD: %w", err)
		}
		jwtCfg.Subject = a.ImpersonateEmail
		svc, err = youtube.NewService(ctx, option.WithHTTPClient(jwtCfg.Client(ctx)))
		if err != nil {
			return nil, fmt.Errorf("create youtube service (DWD): %w", err)
		}
		slog.InfoContext(ctx, "youtube auth initialized", "mode", "SA DWD", "subject", a.ImpersonateEmail)

	case PersonalYouTubeAuth:
		// Personal mode: OAuth2 refresh token representing the channel owner.
		cfg := &oauth2.Config{
			ClientID:     a.ClientID,
			ClientSecret: a.ClientSecret,
			Endpoint:     google.Endpoint,
			Scopes:       []string{youtube.YoutubeForceSslScope},
		}
		ts := cfg.TokenSource(context.Background(), &oauth2.Token{RefreshToken: a.RefreshToken})
		svc, err = youtube.NewService(context.Background(), option.WithTokenSource(ts))
		if err != nil {
			return nil, fmt.Errorf("create youtube service: %w", err)
		}
		slog.InfoContext(ctx, "youtube auth initialized", "mode", "OAuth2 refresh token")

	default:
		return nil, fmt.Errorf("unknown YouTubeAuth type: %T", p.auth)
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
