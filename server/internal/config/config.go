// Package config loads and validates application configuration from environment variables.
package config

import (
	"fmt"
	"os"
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
	AgoraNCSSecret string
	// AgoraRegion sets the Agora Media Push API region.
	// Must match the CDN origin location. Values: cn, ap, na, eu.
	AgoraRegion string
	// AgoraTranscodingEnabled controls whether Agora Media Push re-encodes the stream
	// before relaying to RTMP targets. Defaults to false (raw push, no transcoding)
	// to reduce costs, since both GCP and YouTube can accept the raw RTMP stream.
	AgoraTranscodingEnabled bool

	// GCP
	GCPProjectID  string
	GCPRegion     string
	GCPBucketName string
	GCPCDNDomain  string

	// YouTube
	YouTubeClientID     string
	YouTubeClientSecret string
	YouTubeRefreshToken string
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
		AgoraNCSSecret:          os.Getenv("AGORA_NCS_SECRET"),
		AgoraRegion:             getEnv("AGORA_REGION", "ap"),
		AgoraTranscodingEnabled: getEnvBool("AGORA_TRANSCODING_ENABLED", false),
		GCPProjectID:            os.Getenv("GCP_PROJECT_ID"),
		GCPRegion:               getEnv("GCP_REGION", "us-central1"),
		GCPBucketName:           os.Getenv("GCP_BUCKET_NAME"),
		GCPCDNDomain:            os.Getenv("GCP_CDN_DOMAIN"),
		YouTubeClientID:         os.Getenv("YOUTUBE_CLIENT_ID"),
		YouTubeClientSecret:     os.Getenv("YOUTUBE_CLIENT_SECRET"),
		YouTubeRefreshToken:     os.Getenv("YOUTUBE_REFRESH_TOKEN"),
	}

	if err := cfg.validate(); err != nil {
		return nil, err
	}
	return cfg, nil
}

func (c *Config) validate() error {
	required := map[string]string{
		"JWT_SECRET":            c.JWTSecret,
		"AGORA_APP_ID":          c.AgoraAppID,
		"AGORA_APP_CERTIFICATE": c.AgoraAppCertificate,
		"AGORA_REST_KEY":        c.AgoraRESTKey,
		"AGORA_REST_SECRET":     c.AgoraRESTSecret,
		"AGORA_NCS_SECRET":      c.AgoraNCSSecret,
		"GCP_PROJECT_ID":        c.GCPProjectID,
		"GCP_BUCKET_NAME":       c.GCPBucketName,
		"GCP_CDN_DOMAIN":        c.GCPCDNDomain,
		"YOUTUBE_CLIENT_ID":     c.YouTubeClientID,
		"YOUTUBE_CLIENT_SECRET": c.YouTubeClientSecret,
		"YOUTUBE_REFRESH_TOKEN": c.YouTubeRefreshToken,
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

func getEnvBool(key string, fallback bool) bool {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	return v == "true" || v == "1"
}
