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
	svc      *service.LiveService
	agoraNCS provider.AgoraNCSProvider
}

// NewWebhookHandler creates a new WebhookHandler.
func NewWebhookHandler(svc *service.LiveService, agoraNCS provider.AgoraNCSProvider) *WebhookHandler {
	return &WebhookHandler{svc: svc, agoraNCS: agoraNCS}
}

// agoraWebhookPayload represents the incoming Agora NCS event structure.
type agoraWebhookPayload struct {
	NoticeID  string          `json:"noticeId"`
	ProductID int             `json:"productId"`
	EventType int             `json:"eventType"`
	Payload   json.RawMessage `json:"payload"`
}

// agoraWebhookEventPayload extracts the broadcaster UID from the NCS event payload.
type agoraWebhookEventPayload struct {
	UID uint32 `json:"uid"`
}

// HandleAgora handles POST /v1/webhook/agora.
func (h *WebhookHandler) HandleAgora(c *gin.Context) {
	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		c.JSON(http.StatusBadRequest, model.ErrorResponse{
			Error:   "bad_request",
			Message: "failed to read request body",
		})
		return
	}

	// Verify Agora NCS signature
	signature := c.GetHeader("Agora-Signature")
	if signature != "" && !h.agoraNCS.VerifySignature(body, signature) {
		c.JSON(http.StatusUnauthorized, model.ErrorResponse{
			Error:   "unauthorized",
			Message: "invalid webhook signature",
		})
		return
	}

	var payload agoraWebhookPayload
	if err := json.Unmarshal(body, &payload); err != nil {
		c.JSON(http.StatusBadRequest, model.ErrorResponse{
			Error:   "bad_request",
			Message: "invalid JSON payload",
		})
		return
	}

	// Extract broadcaster UID from NCS payload
	var eventPayload agoraWebhookEventPayload
	if len(payload.Payload) > 0 {
		_ = json.Unmarshal(payload.Payload, &eventPayload)
	}

	if err := h.svc.HandleAgoraWebhook(c.Request.Context(), payload.EventType, eventPayload.UID); err != nil {
		c.JSON(http.StatusInternalServerError, model.ErrorResponse{
			Error:   "webhook_failed",
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}
