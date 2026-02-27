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

	// YouTube
	YouTubeClientID     string
	YouTubeClientSecret string
	YouTubeRefreshToken string

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
		GCPProjectID:         os.Getenv("GCP_PROJECT_ID"),
		GCPRegion:            getEnv("GCP_REGION", "us-central1"),
		GCPBucketName:        os.Getenv("GCP_BUCKET_NAME"),
		GCPCDNDomain:         os.Getenv("GCP_CDN_DOMAIN"),
		GCPRelayEnabled:      getEnvBool("GCP_RELAY_ENABLED", true),
		YouTubeClientID:      os.Getenv("YOUTUBE_CLIENT_ID"),
		YouTubeClientSecret:  os.Getenv("YOUTUBE_CLIENT_SECRET"),
		YouTubeRefreshToken:  os.Getenv("YOUTUBE_REFRESH_TOKEN"),
		YouTubeRelayEnabled:  getEnvBool("YOUTUBE_RELAY_ENABLED", true),
		CORSOrigins:          getEnvStringSlice("CORS_ORIGINS", []string{"http://localhost:3000", "http://localhost:3001"}),
		CORSMethods:          getEnvStringSlice("CORS_METHODS", []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"}),
		CORSHeaders:          getEnvStringSlice("CORS_HEADERS", []string{"Origin", "Content-Type", "Authorization"}),
		CORSExposeHeaders:    getEnvStringSlice("CORS_EXPOSE_HEADERS", []string{"Content-Length"}),
		CORSAllowCredentials: getEnvBool("CORS_ALLOW_CREDENTIALS", true),
		CORSMaxAge:           getEnvDurationHours("CORS_MAX_AGE_HOURS", 12*time.Hour),
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
		required["YOUTUBE_CLIENT_ID"] = c.YouTubeClientID
		required["YOUTUBE_CLIENT_SECRET"] = c.YouTubeClientSecret
		required["YOUTUBE_REFRESH_TOKEN"] = c.YouTubeRefreshToken
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
