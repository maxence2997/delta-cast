package provider

import (
	"crypto/hmac"
	"crypto/sha1"
	"encoding/hex"
)

// --- RTC Channel NCS (Console → Notifications → RTC Channel Event Callbacks) ---

type agoraChannelNCSProvider struct {
	secret string
}

// NewAgoraChannelNCSProvider creates a provider that verifies RTC Channel Event Callback signatures.
func NewAgoraChannelNCSProvider(secret string) AgoraChannelNCSProvider {
	return &agoraChannelNCSProvider{secret: secret}
}

// VerifySignature validates the Agora RTC Channel NCS webhook signature (HMAC/SHA1).
func (p *agoraChannelNCSProvider) VerifySignature(body []byte, signature string) bool {
	mac := hmac.New(sha1.New, []byte(p.secret))
	mac.Write(body)
	expected := hex.EncodeToString(mac.Sum(nil))
	return hmac.Equal([]byte(expected), []byte(signature))
}

// --- Media Push NCS (Console → Notifications → Media Push Restful API) ---

type agoraMediaPushNCSProvider struct {
	secret string
}

// NewAgoraMediaPushNCSProvider creates a provider that verifies Media Push notification signatures.
func NewAgoraMediaPushNCSProvider(secret string) AgoraMediaPushNCSProvider {
	return &agoraMediaPushNCSProvider{secret: secret}
}

// VerifySignature validates the Agora Media Push NCS webhook signature (HMAC/SHA1).
func (p *agoraMediaPushNCSProvider) VerifySignature(body []byte, signature string) bool {
	mac := hmac.New(sha1.New, []byte(p.secret))
	mac.Write(body)
	expected := hex.EncodeToString(mac.Sum(nil))
	return hmac.Equal([]byte(expected), []byte(signature))
}
