// Package provider encapsulates all third-party API interactions.
package provider

import "context"

// AgoraTokenProvider generates Agora RTC tokens.
type AgoraTokenProvider interface {
	// GenerateRTCToken creates an RTC token for the given channel and UID.
	GenerateRTCToken(channelName string, uid uint32, ttlSeconds uint32) (string, error)
}

// ConverterInfo holds the identity of an Agora Media Push converter.
type ConverterInfo struct {
	ID   string
	Name string
}

// AgoraMediaPushProvider manages Agora Media Push (RTMP converter) operations.
type AgoraMediaPushProvider interface {
	// StartMediaPush creates a converter that pushes the specified user's stream to the
	// given RTMP URL. name must be unique within the project (max 64 chars).
	// uid is the Agora RTC UID whose stream should be forwarded (required for non-transcoded mode).
	// Returns the converter ID for stopping later.
	StartMediaPush(ctx context.Context, name string, channelName string, uid uint32, rtmpURL string) (string, error)
	// StopMediaPush destroys a converter by its ID. Returns nil if the converter no
	// longer exists (idempotent).
	StopMediaPush(ctx context.Context, converterID string) error
	// ListConvertersByChannel returns all active converters for the given channel.
	ListConvertersByChannel(ctx context.Context, channelName string) ([]ConverterInfo, error)
}

// AgoraChannelNCSProvider verifies webhook signatures for RTC Channel Event Callbacks.
// Corresponds to the secret under Console → Notifications → RTC Channel Event Callbacks.
type AgoraChannelNCSProvider interface {
	// VerifySignature validates the Agora RTC Channel NCS webhook signature (HMAC/SHA1).
	VerifySignature(body []byte, signature string) bool
}

// AgoraMediaPushNCSProvider verifies webhook signatures for Media Push Restful API notifications.
// Corresponds to the secret under Console → Notifications → Media Push Restful API.
type AgoraMediaPushNCSProvider interface {
	// VerifySignature validates the Agora Media Push NCS webhook signature (HMAC/SHA1).
	VerifySignature(body []byte, signature string) bool
}

// ChannelInfo holds the identity and current state of a GCP Live Stream channel.
type ChannelInfo struct {
	// ID is the short channel identifier (e.g. "channel-3710b194"), not the full resource name.
	ID string
	// StreamingState is the current state as returned by the API
	// (e.g. "AWAITING_INPUT", "STREAMING", "STOPPED", "STOPPING").
	StreamingState string
}

// GCPLiveStreamProvider manages Google Cloud Live Stream API resources.
type GCPLiveStreamProvider interface {
	// CreateInput creates an RTMP input endpoint and returns (inputID, rtmpURI).
	CreateInput(ctx context.Context, inputID string) (string, string, error)
	// CreateChannel creates a live stream channel attached to the given input.
	// Returns the channelID.
	CreateChannel(ctx context.Context, channelID string, inputID string) (string, error)
	// StartChannel starts a live stream channel.
	StartChannel(ctx context.Context, channelID string) error
	// StopChannel stops a live stream channel.
	StopChannel(ctx context.Context, channelID string) error
	// DeleteChannel deletes a live stream channel.
	DeleteChannel(ctx context.Context, channelID string) error
	// DeleteInput deletes an RTMP input endpoint.
	DeleteInput(ctx context.Context, inputID string) error
	// GetPlaybackURL returns the HLS playback URL for the given channel.
	GetPlaybackURL(channelID string) string
	// WaitForChannelReady polls until the channel is in AWAITING_INPUT state.
	WaitForChannelReady(ctx context.Context, channelID string) error
	// ListChannels returns all channels in the configured region.
	// Used at startup for orphan recovery.
	ListChannels(ctx context.Context) ([]ChannelInfo, error)
}

// YouTubeProvider manages YouTube Data API v3 operations.
type YouTubeProvider interface {
	// CreateBroadcast creates a YouTube live broadcast and returns the broadcast ID.
	CreateBroadcast(ctx context.Context, title string) (string, error)
	// CreateStream creates a YouTube live stream and returns (streamID, rtmpURL, streamKey).
	CreateStream(ctx context.Context) (string, string, string, error)
	// BindBroadcastToStream binds a broadcast to a stream.
	BindBroadcastToStream(ctx context.Context, broadcastID string, streamID string) error
	// TransitionBroadcast transitions a broadcast to the given status (testing, live, complete).
	TransitionBroadcast(ctx context.Context, broadcastID string, status string) error
	// GetWatchURL returns the YouTube watch URL for a broadcast.
	GetWatchURL(broadcastID string) string
}
