package http

import (
	"io"
	"net/http"
	"strconv"

	"attendance-backend/internal/entity"
	"attendance-backend/internal/usecase"
	"github.com/gin-gonic/gin"
)

type EnrollmentHandler struct {
	enrollmentUC *usecase.EnrollmentUseCase
}

func NewEnrollmentHandler(enrollmentUC *usecase.EnrollmentUseCase) *EnrollmentHandler {
	return &EnrollmentHandler{enrollmentUC: enrollmentUC}
}

func (h *EnrollmentHandler) Pending(c *gin.Context) {
	jobs, err := h.enrollmentUC.GetPendingJobs(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, jobs)
}

func (h *EnrollmentHandler) Create(c *gin.Context) {
	userID := c.PostForm("user_id")
	if userID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "user_id is required"})
		return
	}

	file, header, err := c.Request.FormFile("image")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "image file is required"})
		return
	}
	defer file.Close()

	fileBytes, err := io.ReadAll(file)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to read image"})
		return
	}

	job, err := h.enrollmentUC.CreateJobFromImage(c.Request.Context(), userID, fileBytes, header.Filename)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, job)
}

func (h *EnrollmentHandler) Claim(c *gin.Context) {
	device := c.MustGet(ContextDeviceKey).(*entity.Device)

	resp, err := h.enrollmentUC.ClaimPendingJob(c.Request.Context(), device.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, resp)
}

type enrollmentResultRequest struct {
	JobID        string  `json:"job_id" binding:"required"`
	EmbeddingB64 string  `json:"embedding_b64" binding:"required"`
	QualityScore float64 `json:"quality_score"`
}

func (h *EnrollmentHandler) Result(c *gin.Context) {
	device := c.MustGet(ContextDeviceKey).(*entity.Device)

	var req enrollmentResultRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.enrollmentUC.SubmitJobResult(c.Request.Context(), req.JobID, device.ID, req.EmbeddingB64, req.QualityScore); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

// ---------------- Audit Handler ----------------

type AuditHandler struct {
	auditUC *usecase.AuditUseCase
}

func NewAuditHandler(auditUC *usecase.AuditUseCase) *AuditHandler {
	return &AuditHandler{auditUC: auditUC}
}

func (h *AuditHandler) List(c *gin.Context) {
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "100"))
	logs, err := h.auditUC.List(c.Request.Context(), limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, logs)
}
