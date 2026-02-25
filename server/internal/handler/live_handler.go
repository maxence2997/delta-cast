// Package handler contains HTTP request handlers for the API.
package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/maxence2997/delta-cast/server/internal/model"
	"github.com/maxence2997/delta-cast/server/internal/service"
)

// LiveHandler handles live streaming API endpoints.
type LiveHandler struct {
	svc   *service.LiveService
	appID string
}

// NewLiveHandler creates a new LiveHandler.
func NewLiveHandler(svc *service.LiveService, appID string) *LiveHandler {
	return &LiveHandler{svc: svc, appID: appID}
}

// Prepare handles POST /v1/live/prepare.
func (h *LiveHandler) Prepare(c *gin.Context) {
	resp, err := h.svc.Prepare(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, model.ErrorResponse{
			Error:   "prepare_failed",
			Message: err.Error(),
		})
		return
	}
	c.JSON(http.StatusOK, resp)
}

// Start handles POST /v1/live/start.
func (h *LiveHandler) Start(c *gin.Context) {
	resp, err := h.svc.Start(c.Request.Context(), h.appID)
	if err != nil {
		c.JSON(http.StatusBadRequest, model.ErrorResponse{
			Error:   "start_failed",
			Message: err.Error(),
		})
		return
	}
	c.JSON(http.StatusOK, resp)
}

// Stop handles POST /v1/live/stop.
func (h *LiveHandler) Stop(c *gin.Context) {
	resp, err := h.svc.Stop(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, model.ErrorResponse{
			Error:   "stop_failed",
			Message: err.Error(),
		})
		return
	}
	c.JSON(http.StatusOK, resp)
}

// Status handles GET /v1/live/status.
func (h *LiveHandler) Status(c *gin.Context) {
	resp := h.svc.Status()
	c.JSON(http.StatusOK, resp)
}
