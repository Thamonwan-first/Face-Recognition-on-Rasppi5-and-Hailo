package http

import (
	"net/http"
	"strconv"

	"attendance-backend/internal/usecase"
	"github.com/gin-gonic/gin"
)

type AttendanceHandler struct {
	attendanceUC *usecase.AttendanceUseCase
}

func NewAttendanceHandler(attendanceUC *usecase.AttendanceUseCase) *AttendanceHandler {
	return &AttendanceHandler{attendanceUC: attendanceUC}
}

func (h *AttendanceHandler) ListPairs(c *gin.Context) {
	start := c.Query("start")
	end := c.Query("end")
	userID := c.Query("user_id")
	dept := c.Query("department")
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "300"))

	pairs, err := h.attendanceUC.ListPairs(c.Request.Context(), start, end, userID, dept, limit)
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
