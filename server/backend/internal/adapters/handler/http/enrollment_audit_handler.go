package http

import (
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"strconv"
	"time"

	"attendance-backend/internal/entity"
	"attendance-backend/internal/pkg/broker"
	"attendance-backend/internal/pkg/token"
	"attendance-backend/internal/ports"
	"attendance-backend/internal/usecase"
	"github.com/gin-gonic/gin"
)

type EnrollmentHandler struct {
	enrollmentUC *usecase.EnrollmentUseCase
	userRepo     ports.UserRepository
	auditRepo    ports.AuditRepository
	broker       *broker.EventBroker
}

func NewEnrollmentHandler(enrollmentUC *usecase.EnrollmentUseCase, extras ...any) *EnrollmentHandler {
	h := &EnrollmentHandler{enrollmentUC: enrollmentUC}
	for _, extra := range extras {
		switch v := extra.(type) {
		case *broker.EventBroker:
			h.broker = v
		case ports.AuditRepository:
			h.auditRepo = v
		case ports.UserRepository:
			h.userRepo = v
		}
	}
	return h
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

	var files []*multipart.FileHeader
	form, formErr := c.MultipartForm()
	if formErr == nil && form != nil {
		if fList, ok := form.File["images"]; ok && len(fList) > 0 {
			files = fList
		} else if fList, ok := form.File["image"]; ok && len(fList) > 0 {
			files = fList
		}
	}
	if len(files) == 0 {
		file, header, err := c.Request.FormFile("image")
		if err == nil {
			_ = file.Close()
			files = []*multipart.FileHeader{header}
		}
	}

	if len(files) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "image file is required"})
		return
	}

	var imagesData [][]byte
	var filenames []string
	for _, fh := range files {
		f, err := fh.Open()
		if err != nil {
			continue
		}
		data, err := io.ReadAll(f)
		_ = f.Close()
		if err == nil && len(data) > 0 {
			imagesData = append(imagesData, data)
			filenames = append(filenames, fh.Filename)
		}
	}

	if len(imagesData) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "no valid image data received"})
		return
	}

	job, err := h.enrollmentUC.CreateJobFromImages(c.Request.Context(), userID, imagesData, filenames)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if h.broker != nil {
		h.broker.Publish(broker.Event{
			Type:    "enrollment",
			Payload: gin.H{"action": "job_created", "job_id": job.ID},
		})
	}

	// Audit log: บันทึกทันทีเมื่อแอดมินส่งคำขอลงทะเบียนใบหน้า
	if h.auditRepo != nil {
		adminID := ""
		if claimsVal, exists := c.Get(ContextAdminKey); exists {
			if claims, ok := claimsVal.(*token.Claims); ok {
				adminID = claims.AdminID
			}
		}
		empInfo := userID
		if h.userRepo != nil {
			if u, err := h.userRepo.GetByID(c.Request.Context(), userID); err == nil && u != nil {
				if u.EmployeeID != "" {
					empInfo = fmt.Sprintf("[%s] %s", u.EmployeeID, u.Name)
				} else {
					empInfo = u.Name
				}
			}
		}
		detailMsg := fmt.Sprintf("ส่งรูปถ่ายลงทะเบียนใบหน้าพนักงาน: %s (%d รูปภาพ) ไปยังตู้ Kiosk", empInfo, len(imagesData))
		_ = h.auditRepo.Log(c.Request.Context(), &entity.AuditLog{
			AdminID:    &adminID,
			Action:     "ลงทะเบียนใบหน้า",
			TargetType: "user",
			TargetID:   userID,
			Detail:     detailMsg,
			CreatedAt:  time.Now().UTC(),
		})
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
	ImagesCount  int     `json:"images_count"`
}

func (h *EnrollmentHandler) Result(c *gin.Context) {
	device := c.MustGet(ContextDeviceKey).(*entity.Device)

	var req enrollmentResultRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.enrollmentUC.SubmitJobResult(c.Request.Context(), req.JobID, device.ID, req.EmbeddingB64, req.QualityScore, req.ImagesCount); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if h.broker != nil {
		h.broker.Publish(broker.Event{
			Type:    "enrollment",
			Payload: gin.H{"action": "result_submitted", "job_id": req.JobID},
		})
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
