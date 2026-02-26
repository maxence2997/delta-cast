// Package provider encapsulates all third-party API interactions.
package provider

import "context"

// AgoraTokenProvider generates Agora RTC tokens.
type AgoraTokenProvider interface {
	// GenerateRTCToken creates an RTC token for the given channel and UID.
	GenerateRTCToken(channelName string, uid uint32, ttlSeconds uint32) (string, error)
}

// AgoraMediaPushProvider manages Agora Media Push (RTMP converter) operations.
type AgoraMediaPushProvider interface {
	// StartMediaPush starts pushing the specified user's stream to the given RTMP URL.
	// uid is the Agora RTC UID whose stream should be forwarded (required for non-transcoded mode).
	// Returns the converter ID for stopping later.
	StartMediaPush(ctx context.Context, channelName string, uid uint32, rtmpURL string) (string, error)
	// StopMediaPush destroys a converter by its ID.
	StopMediaPush(ctx context.Context, converterID string) error
}

// AgoraNCSProvider handles Agora Notification Callback Service signature verification.
type AgoraNCSProvider interface {
	// VerifySignature validates the Agora NCS webhook signature.
	VerifySignature(body []byte, signature string) bool
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
