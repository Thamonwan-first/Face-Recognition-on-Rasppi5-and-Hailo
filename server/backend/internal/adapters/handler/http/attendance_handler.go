package http

import (
	"net/http"
	"strconv"

	"attendance-backend/internal/pkg/broker"
	"attendance-backend/internal/pkg/token"
	"attendance-backend/internal/usecase"
	"github.com/gin-gonic/gin"
)

type AttendanceHandler struct {
	attendanceUC *usecase.AttendanceUseCase
	broker       *broker.EventBroker
}

func NewAttendanceHandler(attendanceUC *usecase.AttendanceUseCase, b ...*broker.EventBroker) *AttendanceHandler {
	var brokerInst *broker.EventBroker
	if len(b) > 0 {
		brokerInst = b[0]
	}
	return &AttendanceHandler{attendanceUC: attendanceUC, broker: brokerInst}
}

func (h *AttendanceHandler) ListPairs(c *gin.Context) {
	start := c.Query("start")
	end := c.Query("end")
	userID := c.Query("user_id")
	dept := c.Query("department")
	deviceID := c.Query("device_id")
	includeAbsent := c.DefaultQuery("include_absent", "true") == "true"
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "500"))

	pairs, err := h.attendanceUC.ListPairs(c.Request.Context(), start, end, userID, dept, deviceID, includeAbsent, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, pairs)
}

func (h *AttendanceHandler) ListEvents(c *gin.Context) {
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "100"))
	events, err := h.attendanceUC.ListEvents(c.Request.Context(), limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, events)
}

func (h *AttendanceHandler) ManualAdd(c *gin.Context) {
	claimsVal, exists := c.Get(ContextAdminKey)
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}
	claims := claimsVal.(*token.Claims)

	var dto usecase.ManualAttendanceDTO
	if err := c.ShouldBindJSON(&dto); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	pair, err := h.attendanceUC.ManualAddAttendance(c.Request.Context(), claims.AdminID, &dto)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if h.broker != nil {
		h.broker.Publish(broker.Event{
			Type: "attendance",
			Payload: gin.H{
				"action":  "manual_add",
				"user_id": dto.UserID,
				"pair_id": pair.ID,
			},
		})
	}

	c.JSON(http.StatusCreated, pair)
}

func (h *AttendanceHandler) ManualEdit(c *gin.Context) {
	claimsVal, exists := c.Get(ContextAdminKey)
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}
	claims := claimsVal.(*token.Claims)

	pairID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid pair id"})
		return
	}

	var dto usecase.UpdateAttendancePairDTO
	if err := c.ShouldBindJSON(&dto); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	pair, err := h.attendanceUC.ManualEditAttendance(c.Request.Context(), claims.AdminID, pairID, &dto)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if h.broker != nil {
		h.broker.Publish(broker.Event{
			Type: "attendance",
			Payload: gin.H{
				"action":  "manual_edit",
				"pair_id": pair.ID,
			},
		})
	}

	c.JSON(http.StatusOK, pair)
}

func (h *AttendanceHandler) ListUnrecognized(c *gin.Context) {
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "100"))
	list, err := h.attendanceUC.ListUnrecognized(c.Request.Context(), limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, list)
}

type ResolveUnrecognizedDTO struct {
	UserID    string `json:"user_id"`
	Action    string `json:"action"` // "assign" or "dismiss"
	Direction string `json:"direction"`
	WorkDate  string `json:"work_date"`
	ScanTime  string `json:"scan_time"`
}

func (h *AttendanceHandler) ResolveUnrecognized(c *gin.Context) {
	claimsVal, exists := c.Get(ContextAdminKey)
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}
	claims := claimsVal.(*token.Claims)
	id := c.Param("id")

	var dto ResolveUnrecognizedDTO
	if err := c.ShouldBindJSON(&dto); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if dto.Action == "" && dto.UserID != "" {
		dto.Action = "assign"
	}

	if err := h.attendanceUC.ResolveUnrecognized(c.Request.Context(), claims.AdminID, id, dto.UserID, dto.Action, dto.Direction, dto.WorkDate, dto.ScanTime); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if h.broker != nil {
		h.broker.Publish(broker.Event{
			Type: "attendance",
			Payload: gin.H{
				"action":          "resolve_unrecognized",
				"unrecognized_id": id,
				"user_id":         dto.UserID,
			},
		})
	}

	c.JSON(http.StatusOK, gin.H{"status": "ok", "id": id})
}

func (h *AttendanceHandler) DeleteUnrecognized(c *gin.Context) {
	id := c.Param("id")
	if err := h.attendanceUC.DeleteUnrecognized(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if h.broker != nil {
		h.broker.Publish(broker.Event{
			Type: "attendance",
			Payload: gin.H{
				"action":          "delete_unrecognized",
				"unrecognized_id": id,
			},
		})
	}
	c.JSON(http.StatusOK, gin.H{"status": "deleted", "id": id})
}

func (h *AttendanceHandler) CreateUserFromUnrecognized(c *gin.Context) {
	claimsVal, exists := c.Get(ContextAdminKey)
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}
	claims := claimsVal.(*token.Claims)
	id := c.Param("id")

	var dto usecase.CreateUserFromScanDTO
	if err := c.ShouldBindJSON(&dto); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	user, err := h.attendanceUC.CreateUserAndResolveUnrecognized(c.Request.Context(), claims.AdminID, id, &dto)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if h.broker != nil {
		h.broker.Publish(broker.Event{
			Type: "attendance",
			Payload: gin.H{
				"action":          "resolve_unrecognized_new_user",
				"unrecognized_id": id,
				"user_id":         user.ID,
			},
		})
		h.broker.Publish(broker.Event{
			Type: "users",
			Payload: gin.H{
				"action":  "created",
				"user_id": user.ID,
			},
		})
	}

	c.JSON(http.StatusOK, gin.H{"status": "ok", "user": user})
}
