package http

import (
	"net/http"

	"attendance-backend/internal/pkg/token"
	"attendance-backend/internal/pkg/broker"
	"attendance-backend/internal/usecase"
	"github.com/gin-gonic/gin"
)

type MeetingHandler struct {
	meetingUC *usecase.MeetingUseCase
	broker    *broker.EventBroker
}

func NewMeetingHandler(meetingUC *usecase.MeetingUseCase, b ...*broker.EventBroker) *MeetingHandler {
	var brokerInst *broker.EventBroker
	if len(b) > 0 {
		brokerInst = b[0]
	}
	return &MeetingHandler{
		meetingUC: meetingUC,
		broker:    brokerInst,
	}
}

func (h *MeetingHandler) List(c *gin.Context) {
	date := c.Query("date")
	status := c.Query("status")
	deviceID := c.Query("device_id")

	meetings, err := h.meetingUC.List(c.Request.Context(), date, status, deviceID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, meetings)
}

func (h *MeetingHandler) Get(c *gin.Context) {
	id := c.Param("id")
	m, err := h.meetingUC.GetByID(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "ไม่พบการประชุม"})
		return
	}
	c.JSON(http.StatusOK, m)
}

func (h *MeetingHandler) Create(c *gin.Context) {
	var dto usecase.CreateMeetingDTO
	if err := c.ShouldBindJSON(&dto); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "ข้อมูลไม่ถูกต้อง: " + err.Error()})
		return
	}

	claims := c.MustGet(ContextAdminKey).(*token.Claims)
	m, err := h.meetingUC.Create(c.Request.Context(), claims.AdminID, &dto)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if h.broker != nil && m != nil {
		h.broker.Publish(broker.Event{
			Type:    "meeting",
			Payload: gin.H{"action": "created", "meeting_id": m.ID, "title": m.Title},
		})
	}

	c.JSON(http.StatusCreated, m)
}

func (h *MeetingHandler) Update(c *gin.Context) {
	id := c.Param("id")
	var dto usecase.UpdateMeetingDTO
	if err := c.ShouldBindJSON(&dto); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "ข้อมูลไม่ถูกต้อง: " + err.Error()})
		return
	}

	claims := c.MustGet(ContextAdminKey).(*token.Claims)
	m, err := h.meetingUC.Update(c.Request.Context(), claims.AdminID, id, &dto)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if h.broker != nil && m != nil {
		h.broker.Publish(broker.Event{
			Type:    "meeting",
			Payload: gin.H{"action": "updated", "meeting_id": m.ID, "title": m.Title},
		})
	}

	c.JSON(http.StatusOK, m)
}

func (h *MeetingHandler) Delete(c *gin.Context) {
	id := c.Param("id")
	claims := c.MustGet(ContextAdminKey).(*token.Claims)
	if err := h.meetingUC.Delete(c.Request.Context(), claims.AdminID, id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if h.broker != nil {
		h.broker.Publish(broker.Event{
			Type:    "meeting",
			Payload: gin.H{"action": "deleted", "meeting_id": id},
		})
	}

	c.JSON(http.StatusOK, gin.H{"status": "deleted"})
}

func (h *MeetingHandler) ListAttendees(c *gin.Context) {
	id := c.Param("id")
	attendees, err := h.meetingUC.ListAttendees(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, attendees)
}
