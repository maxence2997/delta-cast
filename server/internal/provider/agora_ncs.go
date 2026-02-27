package provider

import (
	"crypto/hmac"
	"crypto/sha1"
	"encoding/hex"
)

// agoraNCSProvider is a shared implementation for all Agora NCS signature verification.
// It satisfies both AgoraChannelNCSProvider and AgoraMediaPushNCSProvider since both
// use the same HMAC/SHA1 mechanism, differentiated only by the configured secret.
type agoraNCSProvider struct {
	secret string
}

// VerifySignature validates an Agora NCS webhook signature using HMAC/SHA1.
func (p *agoraNCSProvider) VerifySignature(body []byte, signature string) bool {
	mac := hmac.New(sha1.New, []byte(p.secret))
	mac.Write(body)
	expected := hex.EncodeToString(mac.Sum(nil))
	return hmac.Equal([]byte(expected), []byte(signature))
}

// NewAgoraChannelNCSProvider creates a provider that verifies RTC Channel Event Callback signatures.
// Corresponds to the secret under Console → Notifications → RTC Channel Event Callbacks.
func NewAgoraChannelNCSProvider(secret string) AgoraChannelNCSProvider {
	return &agoraNCSProvider{secret: secret}
}

// NewAgoraMediaPushNCSProvider creates a provider that verifies Media Push notification signatures.
// Corresponds to the secret under Console → Notifications → Media Push Restful API.
func NewAgoraMediaPushNCSProvider(secret string) AgoraMediaPushNCSProvider {
	return &agoraNCSProvider{secret: secret}
}
