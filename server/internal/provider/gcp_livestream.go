package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"time"

	livestreamapi "cloud.google.com/go/video/livestream/apiv1"
	livestreampb "cloud.google.com/go/video/livestream/apiv1/livestreampb"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/impersonate"
	"google.golang.org/api/iterator"
	"google.golang.org/api/option"
	"google.golang.org/protobuf/types/known/durationpb"
)

type gcpLiveStreamProvider struct {
	projectID          string
	region             string
	bucketName         string
	cdnDomain          string
	saKeyPath          string
	saKeyJSON          string
	saImpersonateEmail string
	client             *livestreamapi.Client
}

// NewGCPLiveStreamProvider creates a new GCPLiveStreamProvider.
//
// saKeyPath is a file path to a GCP credential JSON file (GCP_SA_KEY_PATH).
// It accepts any credential format supported by ADC: SA key, Workload Identity Federation
// external_account config, or authorized_user config.
//
// saKeyJSON is the full JSON content of a GCP credential file (GCP_SA_KEY_JSON).
// Used as fallback in PaaS environments where file mounting is not available (e.g. Railway).
// Accepts the same credential formats as saKeyPath.
//
// saImpersonateEmail is the email of the service account to impersonate (GCP_SA_IMPERSONATE_EMAIL).
// When set, the base credential (saKeyPath / saKeyJSON / ADC) is used as the source identity
// to obtain short-lived tokens for the target SA via the IAM generateAccessToken API.
// The source identity must have roles/iam.serviceAccountTokenCreator on the target SA.
//
// Leave saKeyPath, saKeyJSON, and saImpersonateEmail all empty to use ADC directly.
func NewGCPLiveStreamProvider(projectID, region, bucketName, cdnDomain, saKeyPath, saKeyJSON, saImpersonateEmail string) GCPLiveStreamProvider {
	return &gcpLiveStreamProvider{
		projectID:          projectID,
		region:             region,
		bucketName:         bucketName,
		cdnDomain:          cdnDomain,
		saKeyPath:          saKeyPath,
		saKeyJSON:          saKeyJSON,
		saImpersonateEmail: saImpersonateEmail,
	}
}

func (p *gcpLiveStreamProvider) getClient(ctx context.Context) (*livestreamapi.Client, error) {
	if p.client != nil {
		return p.client, nil
	}

	const scope = "https://www.googleapis.com/auth/cloud-platform"

	// Phase 1: resolve base credentials.
	// Priority (high → low):
	// 1. GCP_SA_KEY_PATH — file path to a credential JSON file. Accepts SA key, WIF
	//    external_account config, or authorized_user config.
	// 2. GCP_SA_KEY_JSON — inline credential JSON; for PaaS that cannot mount files.
	//    Accepts the same formats as GCP_SA_KEY_PATH.
	// 3. ADC — SDK discovers credentials automatically via GOOGLE_APPLICATION_CREDENTIALS,
	//    metadata server, or gcloud. Covers Cloud Run, GKE Workload Identity, and local dev.
	var baseCreds *google.Credentials
	authMode := "ADC"
	if p.saKeyPath != "" {
		jsonData, err := os.ReadFile(p.saKeyPath)
		if err != nil {
			return nil, fmt.Errorf("read gcp credential file: %w", err)
		}
		baseCreds, err = credentialsFromJSON(ctx, jsonData, scope)
		if err != nil {
			return nil, fmt.Errorf("parse gcp credential file: %w", err)
		}
		authMode = "credential file"
	} else if p.saKeyJSON != "" {
		var err error
		baseCreds, err = credentialsFromJSON(ctx, []byte(p.saKeyJSON), scope)
		if err != nil {
			return nil, fmt.Errorf("parse gcp inline credential JSON: %w", err)
		}
		authMode = "inline credential JSON"
	}

	// Phase 2: optionally wrap with SA impersonation.
	// When GCP_SA_IMPERSONATE_EMAIL is set, the base credentials (or ADC) are used as the
	// source identity to obtain short-lived tokens for the target SA via IAM generateAccessToken.
	// The source identity must hold roles/iam.serviceAccountTokenCreator on the target SA.
	var finalOpts []option.ClientOption
	if p.saImpersonateEmail != "" {
		var baseOpts []option.ClientOption
		if baseCreds != nil {
			baseOpts = append(baseOpts, option.WithTokenSource(baseCreds.TokenSource))
		}
		ts, err := impersonate.CredentialsTokenSource(ctx, impersonate.CredentialsConfig{
			TargetPrincipal: p.saImpersonateEmail,
			Scopes:          []string{scope},
		}, baseOpts...)
		if err != nil {
			return nil, fmt.Errorf("create impersonation token source for %s: %w", p.saImpersonateEmail, err)
		}
		finalOpts = append(finalOpts, option.WithTokenSource(ts))
		authMode += " + impersonation " + p.saImpersonateEmail
	} else if baseCreds != nil {
		finalOpts = append(finalOpts, option.WithCredentials(baseCreds))
	}
	// ADC without impersonation: no opts passed — SDK discovers credentials automatically.

	slog.InfoContext(ctx, "gcp auth initialized", "mode", authMode)

	client, err := livestreamapi.NewClient(ctx, finalOpts...)
	if err != nil {
		return nil, fmt.Errorf("create livestream client: %w", err)
	}
	p.client = client
	return client, nil
}

// credentialsFromJSON parses a GCP credential JSON and returns Credentials using
// the type-safe google.CredentialsFromJSONWithTypeAndParams.
// It first detects the "type" field in the JSON and validates it against the known
// credential types accepted by this application before loading, preventing
// unintentional loading of an unexpected credential format.
func credentialsFromJSON(ctx context.Context, jsonData []byte, scope string) (*google.Credentials, error) {
	var header struct {
		Type string `json:"type"`
	}
	if err := json.Unmarshal(jsonData, &header); err != nil {
		return nil, fmt.Errorf("detect credential type: %w", err)
	}

	var credType google.CredentialsType
	switch header.Type {
	case string(google.ServiceAccount):
		credType = google.ServiceAccount
	case string(google.AuthorizedUser):
		credType = google.AuthorizedUser
	case string(google.ExternalAccount):
		credType = google.ExternalAccount
	case string(google.ExternalAccountAuthorizedUser):
		credType = google.ExternalAccountAuthorizedUser
	case string(google.ImpersonatedServiceAccount):
		credType = google.ImpersonatedServiceAccount
	default:
		return nil, fmt.Errorf("unsupported gcp credential type %q", header.Type)
	}

	return google.CredentialsFromJSONWithTypeAndParams(ctx, jsonData, credType, google.CredentialsParams{
		Scopes: []string{scope},
	})
}

func (p *gcpLiveStreamProvider) locationPath() string {
	return fmt.Sprintf("projects/%s/locations/%s", p.projectID, p.region)
}

func (p *gcpLiveStreamProvider) inputPath(inputID string) string {
	return fmt.Sprintf("%s/inputs/%s", p.locationPath(), inputID)
}

func (p *gcpLiveStreamProvider) channelPath(channelID string) string {
	return fmt.Sprintf("%s/channels/%s", p.locationPath(), channelID)
}

// CreateInput creates an RTMP input endpoint and returns (inputID, rtmpURI).
func (p *gcpLiveStreamProvider) CreateInput(ctx context.Context, inputID string) (string, string, error) {
	client, err := p.getClient(ctx)
	if err != nil {
		return "", "", err
	}

	req := &livestreampb.CreateInputRequest{
		Parent:  p.locationPath(),
		InputId: inputID,
		Input: &livestreampb.Input{
			Type: livestreampb.Input_RTMP_PUSH,
		},
	}

	op, err := client.CreateInput(ctx, req)
	if err != nil {
		return "", "", fmt.Errorf("create input: %w", err)
	}

	input, err := op.Wait(ctx)
	if err != nil {
		return "", "", fmt.Errorf("wait for input creation: %w", err)
	}

	var rtmpURI string
	if input.Uri != "" {
		rtmpURI = input.Uri
	}

	return inputID, rtmpURI, nil
}

// CreateChannel creates a live stream channel attached to the given input.
// The channel is configured with H.264/AAC transcoding for HLS output via GCS.
func (p *gcpLiveStreamProvider) CreateChannel(ctx context.Context, channelID string, inputID string) (string, error) {
	client, err := p.getClient(ctx)
	if err != nil {
		return "", err
	}

	outputURI := fmt.Sprintf("gs://%s/%s/", p.bucketName, channelID)

	// segmentDuration is the single source of truth for both GOP duration and HLS segment
	// duration. Keeping them equal ensures every segment starts with an IDR (keyframe),
	// which allows players to begin decoding from any segment without prior context.
	segmentDuration := durationpb.New(2 * time.Second)

	req := &livestreampb.CreateChannelRequest{
		Parent:    p.locationPath(),
		ChannelId: channelID,
		// Channel pipeline overview:
		//
		//   ElementaryStream (raw encoded essence, one per codec)
		//       ├── video-stream  (H.264 video only)
		//       └── audio-stream  (AAC audio only)
		//             ↓ packaged into fmp4 segments (1 elementary stream per MuxStream — fmp4 constraint)
		//   MuxStream (segment files written to GCS)
		//       ├── mux-video  →  contains video-stream only
		//       └── mux-audio  →  contains audio-stream only
		//             ↓ referenced by
		//   Manifest
		//       └── main.m3u8  →  HLS playlist combining mux-video + mux-audio
		//
		Channel: &livestreampb.Channel{
			// InputAttachments binds an input source to this channel.
			// Key is a logical identifier; Input points to the RTMP input endpoint created earlier.
			InputAttachments: []*livestreampb.InputAttachment{
				{
					Key:   "primary-input",
					Input: p.inputPath(inputID),
				},
			},
			// Output is the GCS bucket path where transcoded HLS segment files are written.
			// Format: gs://<bucket>/<channelID>/
			Output: &livestreampb.Channel_Output{
				Uri: outputURI,
			},
			// ElementaryStreams define the raw transcoded essence.
			// Each stream must be either pure video or pure audio — never mixed.
			ElementaryStreams: []*livestreampb.ElementaryStream{
				{
					Key: "video-stream",
					ElementaryStream: &livestreampb.ElementaryStream_VideoStream{
						VideoStream: &livestreampb.VideoStream{
							CodecSettings: &livestreampb.VideoStream_H264{
								H264: &livestreampb.VideoStream_H264CodecSettings{
									Profile:    "high",  // H.264 High Profile — broadly compatible
									BitrateBps: 2500000, // 2.5 Mbps — suitable for 720p
									FrameRate:  30,      // 30 fps
									GopMode: &livestreampb.VideoStream_H264CodecSettings_GopDuration{
										GopDuration: segmentDuration,
									},
									WidthPixels:  1280, // 720p resolution (1280×720)
									HeightPixels: 720,
								},
							},
						},
					},
				},
				{
					Key: "audio-stream",
					ElementaryStream: &livestreampb.ElementaryStream_AudioStream{
						AudioStream: &livestreampb.AudioStream{
							Codec:           "aac",  // AAC — standard HLS audio codec
							BitrateBps:      128000, // 128 kbps — standard stereo quality
							ChannelCount:    2,      // Stereo (left + right)
							SampleRateHertz: 48000,  // 48 kHz — broadcast audio standard
						},
					},
				},
			},
			// MuxStreams package elementary streams into fmp4 segment files stored in GCS.
			// fmp4 constraint: each MuxStream must contain exactly one elementary stream,
			// so video and audio are split into separate MuxStreams.
			MuxStreams: []*livestreampb.MuxStream{
				{
					Key:               "mux-video",
					ElementaryStreams: []string{"video-stream"}, // video only
					SegmentSettings: &livestreampb.SegmentSettings{
						// 2 s is a low-latency setting; shorter = lower latency but higher CDN request rate.
						SegmentDuration: segmentDuration,
					},
				},
				{
					Key:               "mux-audio",
					ElementaryStreams: []string{"audio-stream"}, // audio only
					SegmentSettings: &livestreampb.SegmentSettings{
						SegmentDuration: segmentDuration,
					},
				},
			},
			// Manifests define the HLS playlist (.m3u8) file read by the player.
			// The player uses the manifest to discover available segments and merges
			// the separate video and audio tracks during playback.
			Manifests: []*livestreampb.Manifest{
				{
					FileName:   "main.m3u8",                        // playlist filename — last segment of the CDN URL
					Type:       livestreampb.Manifest_HLS,          // HLS (Apple HTTP Live Streaming)
					MuxStreams: []string{"mux-video", "mux-audio"}, // combines both MuxStreams above
					// MaxSegmentCount: sliding window size of the playlist.
					// 5 segments × 2 s = player sees up to the last 10 s of the stream.
					MaxSegmentCount: 5,
					// SegmentKeepDuration: how long GCS retains segment files.
					// 60 s gives slow clients enough time to fetch slightly older segments.
					SegmentKeepDuration: durationpb.New(60 * time.Second),
				},
			},
		},
	}

	op, err := client.CreateChannel(ctx, req)
	if err != nil {
		return "", fmt.Errorf("create channel: %w", err)
	}

	_, err = op.Wait(ctx)
	if err != nil {
		return "", fmt.Errorf("wait for channel creation: %w", err)
	}

	return channelID, nil
}

// StartChannel starts a live stream channel.
func (p *gcpLiveStreamProvider) StartChannel(ctx context.Context, channelID string) error {
	client, err := p.getClient(ctx)
	if err != nil {
		return err
	}

	op, err := client.StartChannel(ctx, &livestreampb.StartChannelRequest{
		Name: p.channelPath(channelID),
	})
	if err != nil {
		return fmt.Errorf("start channel: %w", err)
	}

	_, err = op.Wait(ctx)
	if err != nil {
		return fmt.Errorf("wait for channel start: %w", err)
	}

	return nil
}

// StopChannel stops a live stream channel.
func (p *gcpLiveStreamProvider) StopChannel(ctx context.Context, channelID string) error {
	client, err := p.getClient(ctx)
	if err != nil {
		return err
	}

	op, err := client.StopChannel(ctx, &livestreampb.StopChannelRequest{
		Name: p.channelPath(channelID),
	})
	if err != nil {
		return fmt.Errorf("stop channel: %w", err)
	}

	_, err = op.Wait(ctx)
	if err != nil {
		return fmt.Errorf("wait for channel stop: %w", err)
	}

	return nil
}

// DeleteChannel deletes a live stream channel.
func (p *gcpLiveStreamProvider) DeleteChannel(ctx context.Context, channelID string) error {
	client, err := p.getClient(ctx)
	if err != nil {
		return err
	}

	op, err := client.DeleteChannel(ctx, &livestreampb.DeleteChannelRequest{
		Name: p.channelPath(channelID),
	})
	if err != nil {
		return fmt.Errorf("delete channel: %w", err)
	}

	return op.Wait(ctx)
}

// DeleteInput deletes an RTMP input endpoint.
func (p *gcpLiveStreamProvider) DeleteInput(ctx context.Context, inputID string) error {
	client, err := p.getClient(ctx)
	if err != nil {
		return err
	}

	op, err := client.DeleteInput(ctx, &livestreampb.DeleteInputRequest{
		Name: p.inputPath(inputID),
	})
	if err != nil {
		return fmt.Errorf("delete input: %w", err)
	}

	return op.Wait(ctx)
}

// GetPlaybackURL returns the HLS playback URL via Cloud CDN.
func (p *gcpLiveStreamProvider) GetPlaybackURL(channelID string) string {
	return fmt.Sprintf("https://%s/%s/main.m3u8", p.cdnDomain, channelID)
}

// ListChannels returns all channels in the configured region.
// Used at startup for orphan recovery to clean up channels left active after a crash.
func (p *gcpLiveStreamProvider) ListChannels(ctx context.Context) ([]ChannelInfo, error) {
	client, err := p.getClient(ctx)
	if err != nil {
		return nil, err
	}

	var channels []ChannelInfo
	iter := client.ListChannels(ctx, &livestreampb.ListChannelsRequest{
		Parent: p.locationPath(),
	})
	for {
		ch, err := iter.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("list channels: %w", err)
		}
		// Extract the short channel ID from the full resource name.
		// Full name format: projects/{project}/locations/{location}/channels/{channelID}
		parts := strings.Split(ch.Name, "/")
		channelID := parts[len(parts)-1]
		channels = append(channels, ChannelInfo{
			ID:             channelID,
			StreamingState: ch.StreamingState.String(),
		})
	}
	return channels, nil
}

// WaitForChannelReady polls until the channel is in AWAITING_INPUT or STREAMING state.
func (p *gcpLiveStreamProvider) WaitForChannelReady(ctx context.Context, channelID string) error {
	client, err := p.getClient(ctx)
	if err != nil {
		return err
	}

	timeout := time.After(120 * time.Second)
	ticker := time.NewTicker(3 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-timeout:
			return fmt.Errorf("timeout waiting for channel %s to be ready", channelID)
		case <-ticker.C:
			ch, err := client.GetChannel(ctx, &livestreampb.GetChannelRequest{
				Name: p.channelPath(channelID),
			})
			if err != nil {
				return fmt.Errorf("get channel: %w", err)
			}
			state := ch.StreamingState
			if state == livestreampb.Channel_AWAITING_INPUT || state == livestreampb.Channel_STREAMING {
				return nil
			}
		}
	}
}
