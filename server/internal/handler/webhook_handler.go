package handler

import (
	"encoding/json"
	"io"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/maxence2997/delta-cast/server/internal/model"
	"github.com/maxence2997/delta-cast/server/internal/provider"
	"github.com/maxence2997/delta-cast/server/internal/service"
)

// WebhookHandler handles Agora NCS webhook callbacks.
type WebhookHandler struct {
	svc               *service.LiveService
	agoraChannelNCS   provider.AgoraChannelNCSProvider
	agoraMediaPushNCS provider.AgoraMediaPushNCSProvider
}

// NewWebhookHandler creates a new WebhookHandler.
func NewWebhookHandler(svc *service.LiveService, agoraChannelNCS provider.AgoraChannelNCSProvider, agoraMediaPushNCS provider.AgoraMediaPushNCSProvider) *WebhookHandler {
	return &WebhookHandler{svc: svc, agoraChannelNCS: agoraChannelNCS, agoraMediaPushNCS: agoraMediaPushNCS}
}

// --- Shared ---

// agoraWebhookEnvelope is the common wrapper for all Agora NCS callbacks.
type agoraWebhookEnvelope struct {
	NoticeID  string          `json:"noticeId"`
	ProductID int             `json:"productId"`
	EventType int             `json:"eventType"`
	Payload   json.RawMessage `json:"payload"`
}

// readAndVerifyChannelEvent reads the raw body and verifies the Agora-Signature header.
// Returns (body, true) on success, or writes an error response and returns ("", false).
func readAndVerifyChannelEvent(c *gin.Context, verifier provider.AgoraChannelNCSProvider) ([]byte, bool) {
	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		c.JSON(http.StatusBadRequest, model.ErrorResponse{Error: "bad_request", Message: "failed to read request body"})
		return nil, false
	}
	sig := c.GetHeader("Agora-Signature")
	if sig != "" && !verifier.VerifySignature(body, sig) {
		c.JSON(http.StatusUnauthorized, model.ErrorResponse{Error: "unauthorized", Message: "invalid webhook signature"})
		return nil, false
	}
	return body, true
}

// readAndVerifyMediaPushEvent is the same as readAndVerifyChannelEvent but accepts AgoraMediaPushNCSProvider.
func readAndVerifyMediaPushEvent(c *gin.Context, verifier provider.AgoraMediaPushNCSProvider) ([]byte, bool) {
	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		c.JSON(http.StatusBadRequest, model.ErrorResponse{Error: "bad_request", Message: "failed to read request body"})
		return nil, false
	}
	sig := c.GetHeader("Agora-Signature")
	if sig != "" && !verifier.VerifySignature(body, sig) {
		c.JSON(http.StatusUnauthorized, model.ErrorResponse{Error: "unauthorized", Message: "invalid webhook signature"})
		return nil, false
	}
	return body, true
}

// --- RTC Channel Events ---

// rtcChannelEventPayload extracts the broadcaster UID from the RTC Channel NCS event payload.
type rtcChannelEventPayload struct {
	UID uint32 `json:"uid"`
}

// HandleAgoraChannelEvent handles POST /v1/webhook/agora/channel — RTC Channel Event Callbacks.
// Verifies the signature with AGORA_CHANNEL_NCS_SECRET.
func (h *WebhookHandler) HandleAgoraChannelEvent(c *gin.Context) {
	body, ok := readAndVerifyChannelEvent(c, h.agoraChannelNCS)
	if !ok {
		return
	}

	var envelope agoraWebhookEnvelope
	if err := json.Unmarshal(body, &envelope); err != nil {
		c.JSON(http.StatusBadRequest, model.ErrorResponse{Error: "bad_request", Message: "invalid JSON payload"})
		return
	}

	var eventPayload rtcChannelEventPayload
	if len(envelope.Payload) > 0 {
		_ = json.Unmarshal(envelope.Payload, &eventPayload)
	}

	if err := h.svc.HandleChannelWebhook(c.Request.Context(), envelope.EventType, eventPayload.UID); err != nil {
		c.JSON(http.StatusInternalServerError, model.ErrorResponse{Error: "webhook_failed", Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

// --- Media Push Events ---

// mediaPushConverterPayload extracts the Converter fields from a Media Push NCS event payload.
type mediaPushConverterPayload struct {
	ID    string `json:"id"`
	State string `json:"state"`
}

// mediaPushEventPayload is the top-level payload for Media Push NCS events.
type mediaPushEventPayload struct {
	Converter     mediaPushConverterPayload `json:"converter"`
	DestroyReason string                    `json:"destroyReason"`
}

// HandleAgoraMediaPushEvent handles POST /v1/webhook/agora/media-push — Media Push Restful API notifications.
// Verifies the signature with AGORA_MEDIA_PUSH_NCS_SECRET.
func (h *WebhookHandler) HandleAgoraMediaPushEvent(c *gin.Context) {
	body, ok := readAndVerifyMediaPushEvent(c, h.agoraMediaPushNCS)
	if !ok {
		return
	}

	var envelope agoraWebhookEnvelope
	if err := json.Unmarshal(body, &envelope); err != nil {
		c.JSON(http.StatusBadRequest, model.ErrorResponse{Error: "bad_request", Message: "invalid JSON payload"})
		return
	}

	var eventPayload mediaPushEventPayload
	if len(envelope.Payload) > 0 {
		_ = json.Unmarshal(envelope.Payload, &eventPayload)
	}

	if err := h.svc.HandleMediaPushWebhook(
		c.Request.Context(),
		envelope.EventType,
		eventPayload.Converter.ID,
		eventPayload.Converter.State,
		eventPayload.DestroyReason,
	); err != nil {
		c.JSON(http.StatusInternalServerError, model.ErrorResponse{Error: "webhook_failed", Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}
