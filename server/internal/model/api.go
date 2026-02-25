package model

// PrepareResponse is the response body for POST /v1/live/prepare.
type PrepareResponse struct {
	SessionID string       `json:"sessionId"`
	State     SessionState `json:"state"`
	Message   string       `json:"message"`
}

// StartResponse is the response body for POST /v1/live/start.
type StartResponse struct {
	SessionID    string `json:"sessionId"`
	AgoraAppID   string `json:"agoraAppId"`
	AgoraChannel string `json:"agoraChannel"`
	AgoraToken   string `json:"agoraToken"`
	AgoraUID     uint32 `json:"agoraUid"`
}

// StopResponse is the response body for POST /v1/live/stop.
type StopResponse struct {
	SessionID string       `json:"sessionId"`
	State     SessionState `json:"state"`
	Message   string       `json:"message"`
}

// StatusResponse is the response body for GET /v1/live/status.
type StatusResponse struct {
	SessionID       string       `json:"sessionId"`
	State           SessionState `json:"state"`
	GCPPlaybackURL  string       `json:"gcpPlaybackUrl,omitempty"`
	YouTubeWatchURL string       `json:"youtubeWatchUrl,omitempty"`
}

// ErrorResponse is a standard error response body.
type ErrorResponse struct {
	Error   string `json:"error"`
	Message string `json:"message"`
}
