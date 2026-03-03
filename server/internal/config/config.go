// Package config loads and validates application configuration from environment variables.
package config

import (
	"fmt"
	"os"
	"strings"
	"time"
)

// Config holds all configuration values for the application.
type Config struct {
	ServerPort string

	// JWT
	JWTSecret string

	// Agora
	AgoraAppID          string
	AgoraAppCertificate string
	AgoraRESTKey        string
	AgoraRESTSecret     string
	// AgoraChannelNCSSecret is the secret for RTC Channel Event Callbacks (Console → Notifications → RTC Channel Event Callbacks).
	AgoraChannelNCSSecret string
	// AgoraMediaPushNCSSecret is the secret for Media Push Restful API notifications (Console → Notifications → Media Push Restful API).
	AgoraMediaPushNCSSecret string
	// AgoraRegion sets the Agora Media Push API region.
	// Must match the CDN origin location. Values: cn, ap, na, eu.
	AgoraRegion string
	// AgoraTranscodingEnabled controls whether Agora Media Push re-encodes the stream
	// before relaying to RTMP targets. Defaults to false (raw push, no transcoding)
	// to reduce costs, since both GCP and YouTube can accept the raw RTMP stream.
	AgoraTranscodingEnabled bool

	// Feature flags

	// YouTubeRelayEnabled controls whether YouTube is included as a relay target.
	// Defaults to true. Set to false to skip all YouTube API calls (useful for debugging).
	// When false, YOUTUBE_* env vars are not required.
	YouTubeRelayEnabled bool
	// GCPRelayEnabled controls whether GCP Live Stream is included as a relay target.
	// Defaults to true. Set to false to skip all GCP Live Stream API calls (useful for debugging).
	// When false, GCP_PROJECT_ID, GCP_BUCKET_NAME, and GCP_CDN_DOMAIN are not required.
	GCPRelayEnabled bool

	// GCP
	GCPProjectID  string
	GCPRegion     string
	GCPBucketName string
	GCPCDNDomain  string
	// GCPSAKeyPath is the file path to a GCP credential JSON file (GCP_SA_KEY_PATH).
	// Equivalent to GOOGLE_APPLICATION_CREDENTIALS but uses a GCP_* prefix for consistency.
	// Accepts any format supported by ADC: SA key, Workload Identity Federation
	// external_account config, or authorized_user config.
	// Prefer this over GCPSAKeyJSON when file mounting is available.
	GCPSAKeyPath string
	// GCPSAKeyJSON is the full inline content of a GCP credential JSON file (GCP_SA_KEY_JSON).
	// Fallback after GCPSAKeyPath; used in environments that cannot mount files (e.g. Railway).
	// Accepts the same credential formats as GCPSAKeyPath.
	GCPSAKeyJSON string
	// GCPSAImpersonateEmail is the email of the service account to impersonate (GCP_SA_IMPERSONATE_EMAIL).
	// When set, the base credential (GCPSAKeyPath / GCPSAKeyJSON / ADC) is used as the source
	// identity to obtain short-lived tokens for the target SA via IAM generateAccessToken.
	// The source identity must hold roles/iam.serviceAccountTokenCreator on the target SA.
	// Leave empty to skip impersonation.
	GCPSAImpersonateEmail string

	// YouTube
	YouTubeClientID     string
	YouTubeClientSecret string
	YouTubeRefreshToken string
	// YouTubeImpersonateEmail switches YouTube auth to enterprise mode (SA + DWD).
	// When set, YOUTUBE_CLIENT_ID / SECRET / REFRESH_TOKEN are not required.
	// Leave empty to use personal account mode (OAuth2 refresh token).
	YouTubeImpersonateEmail string
	// YouTubeSAKeyPath is the file path to the SA private key JSON used for YouTube DWD.
	// Only applicable when YouTubeImpersonateEmail is set.
	// Must be a standard SA key JSON — WIF external_account config is not supported for DWD.
	// Falls back to YouTubeSAKeyJSON if empty.
	YouTubeSAKeyPath string
	// YouTubeSAKeyJSON is the full inline content of the SA private key JSON for YouTube DWD.
	// Only applicable when YouTubeImpersonateEmail is set.
	// Used as fallback when file mounting is unavailable (e.g. PaaS).
	YouTubeSAKeyJSON string

	// TrustedProxies is the list of CIDR ranges or IPs that Gin trusts as reverse proxies.
	// Leave nil (unset) for local development. In production behind a Load Balancer,
	// set to the internal IP range, e.g. "10.0.0.0/8" or "169.254.0.0/16" (Cloud Run).
	TrustedProxies []string

	// CORS
	// CORSOrigins is the list of allowed origins for CORS requests.
	// Defaults to localhost:3000 and localhost:3001 for local development.
	CORSOrigins []string
	// CORSMethods is the list of allowed HTTP methods.
	CORSMethods []string
	// CORSHeaders is the list of allowed request headers.
	CORSHeaders []string
	// CORSExposeHeaders is the list of headers exposed to the browser.
	CORSExposeHeaders []string
	// CORSAllowCredentials controls whether credentials (cookies, auth headers) are allowed.
	CORSAllowCredentials bool
	// CORSMaxAge is the duration that preflight responses are cached.
	CORSMaxAge time.Duration
}

// Load reads configuration from environment variables and validates required fields.
func Load() (*Config, error) {
	cfg := &Config{
		ServerPort:              getEnv("SERVER_PORT", "8080"),
		JWTSecret:               os.Getenv("JWT_SECRET"),
		AgoraAppID:              os.Getenv("AGORA_APP_ID"),
		AgoraAppCertificate:     os.Getenv("AGORA_APP_CERTIFICATE"),
		AgoraRESTKey:            os.Getenv("AGORA_REST_KEY"),
		AgoraRESTSecret:         os.Getenv("AGORA_REST_SECRET"),
		AgoraChannelNCSSecret:   os.Getenv("AGORA_CHANNEL_NCS_SECRET"),
		AgoraMediaPushNCSSecret: os.Getenv("AGORA_MEDIA_PUSH_NCS_SECRET"),
		AgoraRegion:             getEnv("AGORA_REGION", "ap"),
		AgoraTranscodingEnabled: getEnvBool("AGORA_TRANSCODING_ENABLED", false),
		GCPProjectID:            os.Getenv("GCP_PROJECT_ID"),
		GCPRegion:               getEnv("GCP_REGION", "us-central1"),
		GCPBucketName:           os.Getenv("GCP_BUCKET_NAME"),
		GCPCDNDomain:            os.Getenv("GCP_CDN_DOMAIN"),
		GCPSAKeyPath:            os.Getenv("GCP_SA_KEY_PATH"),
		GCPSAKeyJSON:            os.Getenv("GCP_SA_KEY_JSON"),
		GCPSAImpersonateEmail:   os.Getenv("GCP_SA_IMPERSONATE_EMAIL"),
		GCPRelayEnabled:         getEnvBool("GCP_RELAY_ENABLED", true),
		YouTubeClientID:         os.Getenv("YOUTUBE_CLIENT_ID"),
		YouTubeClientSecret:     os.Getenv("YOUTUBE_CLIENT_SECRET"),
		YouTubeRefreshToken:     os.Getenv("YOUTUBE_REFRESH_TOKEN"),
		YouTubeImpersonateEmail: os.Getenv("YOUTUBE_IMPERSONATE_EMAIL"),
		YouTubeSAKeyPath:        os.Getenv("YOUTUBE_SA_KEY_PATH"),
		YouTubeSAKeyJSON:        os.Getenv("YOUTUBE_SA_KEY_JSON"),
		YouTubeRelayEnabled:     getEnvBool("YOUTUBE_RELAY_ENABLED", true),
		TrustedProxies:          getEnvStringSlice("TRUSTED_PROXIES", nil),
		CORSOrigins:             getEnvStringSlice("CORS_ORIGINS", []string{"http://localhost:3000", "http://localhost:3001"}),
		CORSMethods:             getEnvStringSlice("CORS_METHODS", []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"}),
		CORSHeaders:             getEnvStringSlice("CORS_HEADERS", []string{"Origin", "Content-Type", "Authorization"}),
		CORSExposeHeaders:       getEnvStringSlice("CORS_EXPOSE_HEADERS", []string{"Content-Length"}),
		CORSAllowCredentials:    getEnvBool("CORS_ALLOW_CREDENTIALS", true),
		CORSMaxAge:              getEnvDurationHours("CORS_MAX_AGE_HOURS", 12*time.Hour),
	}

	if err := cfg.validate(); err != nil {
		return nil, err
	}
	return cfg, nil
}

func (c *Config) validate() error {
	required := map[string]string{
		"JWT_SECRET":                  c.JWTSecret,
		"AGORA_APP_ID":                c.AgoraAppID,
		"AGORA_APP_CERTIFICATE":       c.AgoraAppCertificate,
		"AGORA_REST_KEY":              c.AgoraRESTKey,
		"AGORA_REST_SECRET":           c.AgoraRESTSecret,
		"AGORA_CHANNEL_NCS_SECRET":    c.AgoraChannelNCSSecret,
		"AGORA_MEDIA_PUSH_NCS_SECRET": c.AgoraMediaPushNCSSecret,
	}

	if c.GCPRelayEnabled {
		required["GCP_PROJECT_ID"] = c.GCPProjectID
		required["GCP_BUCKET_NAME"] = c.GCPBucketName
		required["GCP_CDN_DOMAIN"] = c.GCPCDNDomain
	}

	if c.YouTubeRelayEnabled {
		if c.YouTubeImpersonateEmail != "" {
			// Enterprise DWD mode: SA key is required; OAuth2 fields are not needed.
			if c.YouTubeSAKeyPath == "" && c.YouTubeSAKeyJSON == "" {
				return fmt.Errorf("YOUTUBE_IMPERSONATE_EMAIL is set but neither YOUTUBE_SA_KEY_PATH nor YOUTUBE_SA_KEY_JSON is provided (SA private key required for DWD)")
			}
		} else {
			// Personal OAuth2 mode.
			required["YOUTUBE_CLIENT_ID"] = c.YouTubeClientID
			required["YOUTUBE_CLIENT_SECRET"] = c.YouTubeClientSecret
			required["YOUTUBE_REFRESH_TOKEN"] = c.YouTubeRefreshToken
		}
	}

	for key, val := range required {
		if val == "" {
			return fmt.Errorf("required environment variable %s is not set", key)
		}
	}
	return nil
}

func getEnv(key, fallback string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return fallback
}

func getEnvDurationHours(key string, fallback time.Duration) time.Duration {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	hours, err := time.ParseDuration(v + "h")
	if err != nil {
		return fallback
	}
	return hours
}

func getEnvStringSlice(key string, fallback []string) []string {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	parts := strings.Split(v, ",")
	result := make([]string, 0, len(parts))
	for _, p := range parts {
		if trimmed := strings.TrimSpace(p); trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}

func getEnvBool(key string, fallback bool) bool {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	return v == "true" || v == "1"
}
