package provider

import (
	"context"
	"fmt"
	"time"

	livestreamapi "cloud.google.com/go/video/livestream/apiv1"
	livestreampb "cloud.google.com/go/video/livestream/apiv1/livestreampb"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/option"
	"google.golang.org/protobuf/types/known/durationpb"
)

type gcpLiveStreamProvider struct {
	projectID  string
	region     string
	bucketName string
	cdnDomain  string
	saKeyPath  string
	saKeyJSON  string
	client     *livestreamapi.Client
}

// NewGCPLiveStreamProvider creates a new GCPLiveStreamProvider.
// saKeyPath is the file path to a GCP Service Account key JSON file (GCP_SA_KEY_PATH).
// saKeyJSON is the full JSON content of a GCP Service Account key (GCP_SA_KEY_JSON); used as fallback.
// Leave both empty to use Application Default Credentials (ADC).
func NewGCPLiveStreamProvider(projectID, region, bucketName, cdnDomain, saKeyPath, saKeyJSON string) GCPLiveStreamProvider {
	return &gcpLiveStreamProvider{
		projectID:  projectID,
		region:     region,
		bucketName: bucketName,
		cdnDomain:  cdnDomain,
		saKeyPath:  saKeyPath,
		saKeyJSON:  saKeyJSON,
	}
}

func (p *gcpLiveStreamProvider) getClient(ctx context.Context) (*livestreamapi.Client, error) {
	if p.client != nil {
		return p.client, nil
	}
	// Priority:
	// 1. GCP_SA_KEY_PATH — file path to SA key, passed via option.WithCredentialsFile.
	// 2. GCP_SA_KEY_JSON — full JSON content, for PaaS environments that cannot mount files (e.g. Railway).
	// 3. ADC — SDK picks up ambient credentials automatically when neither above is set.
	var opts []option.ClientOption
	if p.saKeyPath != "" {
		opts = append(opts, option.WithCredentialsFile(p.saKeyPath))
	} else if p.saKeyJSON != "" {
		jwtCfg, err := google.JWTConfigFromJSON([]byte(p.saKeyJSON), "https://www.googleapis.com/auth/cloud-platform")
		if err != nil {
			return nil, fmt.Errorf("parse gcp service account key: %w", err)
		}
		opts = append(opts, option.WithTokenSource(jwtCfg.TokenSource(ctx)))
	}
	client, err := livestreamapi.NewClient(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("create livestream client: %w", err)
	}
	p.client = client
	return client, nil
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
