package model

// MediaPushEventType represents Agora Media Push NCS event types (productId=5).
type MediaPushEventType int

const (
	// MediaPushEventConverterCreated is fired when a converter is created via the Create API.
	MediaPushEventConverterCreated MediaPushEventType = 1
	// MediaPushEventConverterConfigChanged is fired when a converter's config is updated via the Update API.
	MediaPushEventConverterConfigChanged MediaPushEventType = 2
	// MediaPushEventConverterStateChanged is fired when a converter's running state changes (e.g. connecting → running/failed).
	MediaPushEventConverterStateChanged MediaPushEventType = 3
	// MediaPushEventConverterDestroyed is fired when a converter is destroyed and RTMP push has stopped.
	MediaPushEventConverterDestroyed MediaPushEventType = 4
)

// ConverterState represents the running state of an Agora Media Push converter,
// delivered in Media Push NCS event type 3 (ConverterStateChanged).
type ConverterState string

const (
	// ConverterStateConnecting means the converter is establishing the RTMP connection.
	ConverterStateConnecting ConverterState = "connecting"
	// ConverterStateRunning means the converter is actively pushing the stream.
	ConverterStateRunning ConverterState = "running"
	// ConverterStateFailed means the stream push to the CDN has failed.
	ConverterStateFailed ConverterState = "failed"
)

// ConverterDestroyReasonInternalError is the destroyReason value Agora sends when
// a converter is destroyed unexpectedly due to an internal error (event type 4).
const ConverterDestroyReasonInternalError = "Internal Error"

// ChannelEventType represents Agora RTC Channel NCS event types (productId=1).
type ChannelEventType int

const (
	// ChannelEventChannelCreate is fired when a channel is created (first user joins).
	ChannelEventChannelCreate ChannelEventType = 101
	// ChannelEventChannelDestroy is fired when a channel is destroyed (last user leaves).
	ChannelEventChannelDestroy ChannelEventType = 102
	// ChannelEventBroadcasterJoin is fired when a host joins in LIVE_BROADCASTING profile.
	ChannelEventBroadcasterJoin ChannelEventType = 103
	// ChannelEventBroadcasterLeave is fired when a host leaves in LIVE_BROADCASTING profile.
	ChannelEventBroadcasterLeave ChannelEventType = 104
	// ChannelEventAudienceJoin is fired when an audience member joins in LIVE_BROADCASTING profile.
	ChannelEventAudienceJoin ChannelEventType = 105
	// ChannelEventAudienceLeave is fired when an audience member leaves in LIVE_BROADCASTING profile.
	ChannelEventAudienceLeave ChannelEventType = 106
	// ChannelEventUserJoinCommunication is fired when a user joins in COMMUNICATION profile.
	ChannelEventUserJoinCommunication ChannelEventType = 107
	// ChannelEventUserLeaveCommunication is fired when a user leaves in COMMUNICATION profile.
	ChannelEventUserLeaveCommunication ChannelEventType = 108
	// ChannelEventRoleToBroadcaster is fired when a user switches role to broadcaster.
	ChannelEventRoleToBroadcaster ChannelEventType = 111
	// ChannelEventRoleToAudience is fired when a user switches role to audience.
	ChannelEventRoleToAudience ChannelEventType = 112
)

// YouTubeBroadcastStatusComplete is the lifecycle status passed to TransitionBroadcast
// when ending a YouTube broadcast via the YouTube Data API.
const YouTubeBroadcastStatusComplete = "complete"
