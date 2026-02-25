package provider

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

const agoraMediaPushBaseURL = "https://api.agora.io/v1/projects/%s/rtmp-converters"

type agoraMediaPushProvider struct {
	appID      string
	restKey    string
	restSecret string
	httpClient *http.Client
}

// NewAgoraMediaPushProvider creates a new AgoraMediaPushProvider.
func NewAgoraMediaPushProvider(appID, restKey, restSecret string) AgoraMediaPushProvider {
	return &agoraMediaPushProvider{
		appID:      appID,
		restKey:    restKey,
		restSecret: restSecret,
		httpClient: &http.Client{},
	}
}

type startPushRequest struct {
	Converter pushConverter `json:"converter"`
}

type pushConverter struct {
	Name             string        `json:"name"`
	TranscodeOptions transcodeOpts `json:"transcodeOptions"`
}

type transcodeOpts struct {
	RtmpURL string    `json:"rtmpUrl"`
	Audio   audioOpts `json:"audioOptions"`
	Video   videoOpts `json:"videoOptions"`
}

type audioOpts struct {
	CodecProfile  string `json:"codecProfile"`
	SampleRate    int    `json:"sampleRate"`
	Bitrate       int    `json:"bitrate"`
	AudioChannels int    `json:"audioChannels"`
}

type videoOpts struct {
	Codec      string `json:"codec"`
	Width      int    `json:"width"`
	Height     int    `json:"height"`
	Fps        int    `json:"fps"`
	Bitrate    int    `json:"bitrate"`
	GopSeconds int    `json:"gopInSec"`
}

type startPushResponse struct {
	Converter struct {
		ID string `json:"id"`
	} `json:"converter"`
}

// StartMediaPush starts pushing the channel stream to the given RTMP URL.
func (p *agoraMediaPushProvider) StartMediaPush(ctx context.Context, channelName string, rtmpURL string) (string, error) {
	reqBody := startPushRequest{
		Converter: pushConverter{
			Name: fmt.Sprintf("%s_converter", channelName),
			TranscodeOptions: transcodeOpts{
				RtmpURL: rtmpURL,
				Audio: audioOpts{
					CodecProfile:  "LC-AAC",
					SampleRate:    48000,
					Bitrate:       128,
					AudioChannels: 2,
				},
				Video: videoOpts{
					Codec:      "H264",
					Width:      1280,
					Height:     720,
					Fps:        30,
					Bitrate:    2500,
					GopSeconds: 2,
				},
			},
		},
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("marshal request: %w", err)
	}

	url := fmt.Sprintf(agoraMediaPushBaseURL, p.appID)
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(bodyBytes))
	if err != nil {
		return "", fmt.Errorf("create request: %w", err)
	}

	p.setHeaders(req)

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("send request: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return "", fmt.Errorf("agora media push start failed: status=%d body=%s", resp.StatusCode, string(respBody))
	}

	var result startPushResponse
	if err := json.Unmarshal(respBody, &result); err != nil {
		return "", fmt.Errorf("unmarshal response: %w", err)
	}

	return result.Converter.ID, nil
}

// StopMediaPush stops a previously started media push by SID.
func (p *agoraMediaPushProvider) StopMediaPush(ctx context.Context, sid string) error {
	url := fmt.Sprintf(agoraMediaPushBaseURL+"/%s", p.appID, sid)
	req, err := http.NewRequestWithContext(ctx, "DELETE", url, nil)
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}

	p.setHeaders(req)

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("agora media push stop failed: status=%d body=%s", resp.StatusCode, string(body))
	}

	return nil
}

func (p *agoraMediaPushProvider) setHeaders(req *http.Request) {
	req.Header.Set("Content-Type", "application/json")
	credentials := base64.StdEncoding.EncodeToString([]byte(p.restKey + ":" + p.restSecret))
	req.Header.Set("Authorization", "Basic "+credentials)
}
