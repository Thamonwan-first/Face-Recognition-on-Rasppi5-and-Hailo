package http

import (
	"net/http"
	"strconv"

	"attendance-backend/internal/entity"
	"attendance-backend/internal/pkg/broker"
	"attendance-backend/internal/usecase"
	"github.com/gin-gonic/gin"
)

type SyncHandler struct {
	syncUC *usecase.SyncUseCase
	broker *broker.EventBroker
}

func NewSyncHandler(syncUC *usecase.SyncUseCase, b ...*broker.EventBroker) *SyncHandler {
	var brokerInst *broker.EventBroker
	if len(b) > 0 {
		brokerInst = b[0]
	}
	return &SyncHandler{syncUC: syncUC, broker: brokerInst}
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

	if h.broker != nil && syncedCount > 0 {
		h.broker.Publish(broker.Event{
			Type: "attendance",
			Payload: gin.H{
				"count":     syncedCount,
				"device_id": device.ID,
			},
		})
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

	if dto.IPAddress == "" {
		dto.IPAddress = c.ClientIP()
	}

	if err := h.syncUC.Heartbeat(c.Request.Context(), device.ID, &dto); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if h.broker != nil {
		h.broker.Publish(broker.Event{
			Type: "device",
			Payload: gin.H{
				"device_id":  device.ID,
				"status":     "online",
				"ip_address": dto.IPAddress,
			},
		})
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

	if h.broker != nil {
		h.broker.Publish(broker.Event{
			Type: "enrollment",
			Payload: gin.H{
				"user_id":   dto.UserID,
				"device_id": device.ID,
			},
		})
	}

	c.JSON(http.StatusOK, gin.H{"status": "ok", "user_id": dto.UserID})
}
