// Package model defines shared data structures used across the application.
package model

import "time"

// SessionState represents the current state of a live session.
type SessionState string

const (
	// StateIdle indicates no active session.
	StateIdle SessionState = "idle"
	// StatePreparing indicates resources are being allocated.
	StatePreparing SessionState = "preparing"
	// StateReady indicates resources are ready to start streaming.
	StateReady SessionState = "ready"
	// StateLive indicates the session is actively streaming.
	StateLive SessionState = "live"
	// StateStopping indicates the session is being torn down.
	StateStopping SessionState = "stopping"
)

// Session holds the in-memory state for a single live streaming session.
type Session struct {
	ID        string       `json:"id"`
	State     SessionState `json:"state"`
	CreatedAt time.Time    `json:"createdAt"`

	// Agora
	AgoraChannel string `json:"agoraChannel,omitempty"`

	// GCP Live Stream
	GCPInputID     string `json:"gcpInputId,omitempty"`
	GCPChannelID   string `json:"gcpChannelId,omitempty"`
	GCPInputURI    string `json:"gcpInputUri,omitempty"`
	GCPPlaybackURL string `json:"gcpPlaybackUrl,omitempty"`

	// YouTube
	YouTubeBroadcastID string `json:"youtubeBroadcastId,omitempty"`
	YouTubeStreamID    string `json:"youtubeStreamId,omitempty"`
	YouTubeStreamKey   string `json:"youtubeStreamKey,omitempty"`
	YouTubeRTMPURL     string `json:"youtubeRtmpUrl,omitempty"`
	YouTubeWatchURL    string `json:"youtubeWatchUrl,omitempty"`

	// Media Push SIDs
	MediaPushGCPSID     string `json:"mediaPushGcpSid,omitempty"`
	MediaPushYouTubeSID string `json:"mediaPushYoutubeSid,omitempty"`

	// NCS deduplication — tracks the clientSeq of the last processed broadcaster-join event
	// (event 103). A later event with an equal or lower clientSeq is a duplicate and is ignored.
	LastBroadcasterClientSeq int64 `json:"-"`
	// SeenNoticeIDs tracks noticeIds already processed in this session to prevent
	// duplicate NCS callbacks from being acted on more than once.
	SeenNoticeIDs map[string]struct{} `json:"-"`
}
