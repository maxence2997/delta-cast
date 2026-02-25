package provider

import (
	"crypto/hmac"
	"crypto/sha1"
	"encoding/hex"
)

type agoraNCSProvider struct {
	secret string
}

// NewAgoraNCSProvider creates a new AgoraNCSProvider.
func NewAgoraNCSProvider(secret string) AgoraNCSProvider {
	return &agoraNCSProvider{secret: secret}
}

// VerifySignature validates the Agora NCS webhook signature.
// Agora signs the raw body with HMAC-SHA1 using the NCS secret.
func (p *agoraNCSProvider) VerifySignature(body []byte, signature string) bool {
	mac := hmac.New(sha1.New, []byte(p.secret))
	mac.Write(body)
	expected := hex.EncodeToString(mac.Sum(nil))
	return hmac.Equal([]byte(expected), []byte(signature))
}
