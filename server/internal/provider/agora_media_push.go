package provider

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/google/uuid"
)

// agoraMediaPushBaseURL includes {region} as required by the API.
// Format: https://api.agora.io/{region}/v1/projects/{appId}/rtmp-converters
const agoraMediaPushBaseURL = "https://api.agora.io/%s/v1/projects/%s/rtmp-converters"

// defaultIdleTimeOut is the max idle seconds before Agora auto-destroys the Converter.
// Idle means all users have left the channel.
const defaultIdleTimeOut = 300

type agoraMediaPushProvider struct {
	appID       string
	region      string
	restKey     string
	restSecret  string
	transcoding bool
	httpClient  *http.Client
}

// NewAgoraMediaPushProvider creates a new AgoraMediaPushProvider.
// region must be one of: cn, ap, na, eu — matching the CDN origin location.
// When transcoding is false (default), Media Push relays the raw stream without
// re-encoding, which reduces costs. Set transcoding to true to enable H.264/AAC
// re-encoding before the RTMP relay.
func NewAgoraMediaPushProvider(appID, region, restKey, restSecret string, transcoding bool) AgoraMediaPushProvider {
	return &agoraMediaPushProvider{
		appID:       appID,
		region:      region,
		restKey:     restKey,
		restSecret:  restSecret,
		transcoding: transcoding,
		httpClient:  &http.Client{},
	}
}

// --- Request / Response types (matches Agora Media Push REST API spec) ---

type createConverterRequest struct {
	Converter converterConfig `json:"converter"`
}

// converterConfig is the top-level converter object.
// rtmpUrl, idleTimeOut, and jitterBufferSizeMs live at this level.
// Exactly one of TranscodeOptions or RawOptions must be set.
type converterConfig struct {
	Name               string         `json:"name"`
	RawOptions         *rawOpts       `json:"rawOptions,omitempty"`
	TranscodeOptions   *transcodeOpts `json:"transcodeOptions,omitempty"`
	RtmpURL            string         `json:"rtmpUrl"`
	IdleTimeOut        int            `json:"idleTimeOut,omitempty"`
	JitterBufferSizeMs int            `json:"jitterBufferSizeMs,omitempty"`
}

// rawOpts is used for non-transcoding (direct relay) mode.
type rawOpts struct {
	RtcChannel   string `json:"rtcChannel"`
	RtcStreamUid uint32 `json:"rtcStreamUid"`
}

// transcodeOpts is used for transcoding (re-encode) mode.
type transcodeOpts struct {
	RtcChannel   string     `json:"rtcChannel"`
	AudioOptions *audioOpts `json:"audioOptions,omitempty"`
	VideoOptions *videoOpts `json:"videoOptions,omitempty"`
}

type audioOpts struct {
	CodecProfile  string `json:"codecProfile"`
	SampleRate    int    `json:"sampleRate"`
	Bitrate       int    `json:"bitrate"`
	AudioChannels int    `json:"audioChannels"`
}

type videoOpts struct {
	Canvas       videoCanvas `json:"canvas"`
	Codec        string      `json:"codec,omitempty"`
	CodecProfile string      `json:"codecProfile,omitempty"`
	FrameRate    int         `json:"frameRate,omitempty"`
	Gop          int         `json:"gop,omitempty"`
	Bitrate      int         `json:"bitrate"`
}

type videoCanvas struct {
	Width  int `json:"width"`
	Height int `json:"height"`
}

type createConverterResponse struct {
	Converter struct {
		ID       string `json:"id"`
		CreateTs int64  `json:"createTs"`
		UpdateTs int64  `json:"updateTs"`
		State    string `json:"state"`
	} `json:"converter"`
	Fields string `json:"fields"`
}

// StartMediaPush creates a Converter that pushes the specified user's stream to the RTMP URL.
// uid is the Agora RTC UID whose stream should be forwarded.
func (p *agoraMediaPushProvider) StartMediaPush(ctx context.Context, channelName string, uid uint32, rtmpURL string) (string, error) {
	converter := converterConfig{
		Name:        fmt.Sprintf("%s_converter", channelName),
		RtmpURL:     rtmpURL,
		IdleTimeOut: defaultIdleTimeOut,
	}

	if p.transcoding {
		converter.TranscodeOptions = &transcodeOpts{
			RtcChannel: channelName,
			AudioOptions: &audioOpts{
				CodecProfile:  "LC-AAC",
				SampleRate:    48000,
				Bitrate:       128,
				AudioChannels: 2,
			},
			VideoOptions: &videoOpts{
				Canvas:       videoCanvas{Width: 1280, Height: 720},
				Codec:        "H.264",
				CodecProfile: "High",
				FrameRate:    30,
				Gop:          60,
				Bitrate:      2500,
			},
		}
	} else {
		// Raw relay: forward the stream as-is without re-encoding.
		converter.RawOptions = &rawOpts{
			RtcChannel:   channelName,
			RtcStreamUid: uid,
		}
	}

	reqBody := createConverterRequest{Converter: converter}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("marshal request: %w", err)
	}

	url := fmt.Sprintf(agoraMediaPushBaseURL, p.region, p.appID)
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

	var result createConverterResponse
	if err := json.Unmarshal(respBody, &result); err != nil {
		return "", fmt.Errorf("unmarshal response: %w", err)
	}

	return result.Converter.ID, nil
}

// StopMediaPush destroys a Converter by its ID.
func (p *agoraMediaPushProvider) StopMediaPush(ctx context.Context, converterID string) error {
	url := fmt.Sprintf(agoraMediaPushBaseURL+"/%s", p.region, p.appID, converterID)
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
	req.Header.Set("X-Request-ID", uuid.New().String())
}
