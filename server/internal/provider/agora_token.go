package provider

import (
	rtctokenbuilder "github.com/AgoraIO/Tools/DynamicKey/AgoraDynamicKey/go/src/rtctokenbuilder2"
)

type agoraTokenProvider struct {
	appID          string
	appCertificate string
}

// NewAgoraTokenProvider creates a new AgoraTokenProvider.
func NewAgoraTokenProvider(appID, appCertificate string) AgoraTokenProvider {
	return &agoraTokenProvider{
		appID:          appID,
		appCertificate: appCertificate,
	}
}

// GenerateRTCToken creates an RTC token for the given channel and UID.
func (p *agoraTokenProvider) GenerateRTCToken(channelName string, uid uint32, ttlSeconds uint32) (string, error) {
	return rtctokenbuilder.BuildTokenWithUid(
		p.appID,
		p.appCertificate,
		channelName,
		uid,
		rtctokenbuilder.RolePublisher,
		ttlSeconds,
		ttlSeconds,
	)
}
