package provider

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/google/uuid"
)

// agoraChannelUserURL queries the list of users currently in a channel.
// Format: https://api.agora.io/dev/v1/channel/user/{appid}/{channelName}?hosts_only=1
const agoraChannelUserURL = "https://api.agora.io/dev/v1/channel/user/%s/%s?hosts_only=1"

type agoraChannelProvider struct {
	appID      string
	restKey    string
	restSecret string
	httpClient *http.Client
}

// NewAgoraChannelProvider creates a provider for the Agora Channel Management REST API.
// restKey and restSecret are the same Agora REST credentials used for Media Push.
func NewAgoraChannelProvider(appID, restKey, restSecret string) AgoraChannelProvider {
	return &agoraChannelProvider{
		appID:      appID,
		restKey:    restKey,
		restSecret: restSecret,
		httpClient: &http.Client{},
	}
}

type queryBroadcastersResponse struct {
	Success bool `json:"success"`
	Data    struct {
		ChannelExist bool     `json:"channel_exist"`
		Mode         int      `json:"mode"`
		Broadcasters []uint32 `json:"broadcasters"`
	} `json:"data"`
}

// QueryBroadcasters returns the UIDs of all hosts currently in the channel and
// whether the channel exists. Uses Agora Channel Management REST API (hosts_only=1).
// Returns (nil, false, nil) when the channel does not exist.
func (p *agoraChannelProvider) QueryBroadcasters(ctx context.Context, channelName string) ([]uint32, bool, error) {
	url := fmt.Sprintf(agoraChannelUserURL, p.appID, channelName)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, false, fmt.Errorf("build request: %w", err)
	}
	p.setHeaders(req)
	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, false, fmt.Errorf("query broadcasters: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, false, fmt.Errorf("query broadcasters failed: status=%d body=%s", resp.StatusCode, string(body))
	}
	var result queryBroadcastersResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, false, fmt.Errorf("decode response: %w", err)
	}
	if !result.Data.ChannelExist {
		return nil, false, nil
	}
	return result.Data.Broadcasters, true, nil
}

func (p *agoraChannelProvider) setHeaders(req *http.Request) {
	req.Header.Set("Content-Type", "application/json")
	credentials := base64.StdEncoding.EncodeToString([]byte(p.restKey + ":" + p.restSecret))
	req.Header.Set("Authorization", "Basic "+credentials)
	req.Header.Set("X-Request-ID", uuid.New().String())
}
