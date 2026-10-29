package http

import (
	"net/http"
	"strconv"

	"attendance-backend/internal/entity"
	"attendance-backend/internal/usecase"
	"github.com/gin-gonic/gin"
)

type SyncHandler struct {
	syncUC *usecase.SyncUseCase
}

func NewSyncHandler(syncUC *usecase.SyncUseCase) *SyncHandler {
	return &SyncHandler{syncUC: syncUC}
}

type syncEventsRequest struct {
	Events []*entity.AttendanceEvent `json:"events" binding:"required"`
}

func (h *SyncHandler) SyncEvents(c *gin.Context) {
	device := c.MustGet(ContextDeviceKey).(*entity.Device)

	var req syncEventsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	syncedCount, err := h.syncUC.ProcessAttendanceEvents(c.Request.Context(), device.ID, req.Events)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status": "ok",
		"synced": syncedCount,
		"total":  len(req.Events),
	})
}

func (h *SyncHandler) Heartbeat(c *gin.Context) {
	device := c.MustGet(ContextDeviceKey).(*entity.Device)

	var dto usecase.HeartbeatDTO
	if err := c.ShouldBindJSON(&dto); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.syncUC.Heartbeat(c.Request.Context(), device.ID, &dto); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

func (h *SyncHandler) SyncUsers(c *gin.Context) {
	sinceStr := c.DefaultQuery("since", "0")
	sinceVersion, _ := strconv.Atoi(sinceStr)

	resp, err := h.syncUC.GetUsersSince(c.Request.Context(), sinceVersion)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, resp)
}

func (h *SyncHandler) KioskEnrollment(c *gin.Context) {
	device := c.MustGet(ContextDeviceKey).(*entity.Device)

	var dto usecase.KioskEnrollmentDTO
	if err := c.ShouldBindJSON(&dto); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.syncUC.SaveKioskEnrollment(c.Request.Context(), device.ID, &dto); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "ok", "user_id": dto.UserID})
}
