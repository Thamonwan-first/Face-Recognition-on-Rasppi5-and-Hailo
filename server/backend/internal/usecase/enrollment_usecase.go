package usecase

import (
	"context"
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"attendance-backend/internal/entity"
	"attendance-backend/internal/ports"
	"github.com/google/uuid"
)

type EnrollmentUseCase struct {
	enrollmentRepo ports.EnrollmentRepository
	userRepo       ports.UserRepository
	auditRepo      ports.AuditRepository
	deviceRepo     ports.DeviceRepository
	uploadDir      string
}

func NewEnrollmentUseCase(enrollmentRepo ports.EnrollmentRepository, userRepo ports.UserRepository, uploadDir string, extras ...any) *EnrollmentUseCase {
	if uploadDir == "" {
		uploadDir = "data/enrollment_images"
	}
	_ = os.MkdirAll(uploadDir, 0755)
	uc := &EnrollmentUseCase{
		enrollmentRepo: enrollmentRepo,
		userRepo:       userRepo,
		uploadDir:      uploadDir,
	}
	for _, extra := range extras {
		switch v := extra.(type) {
		case ports.AuditRepository:
			uc.auditRepo = v
		case ports.DeviceRepository:
			uc.deviceRepo = v
		}
	}
	return uc
}

func (uc *EnrollmentUseCase) GetPendingJobs(ctx context.Context) ([]*entity.EnrollmentJob, error) {
	return uc.enrollmentRepo.GetPendingJobs(ctx)
}

func (uc *EnrollmentUseCase) CreateJobFromImage(ctx context.Context, userID string, imageBytes []byte, filename string) (*entity.EnrollmentJob, error) {
	return uc.CreateJobFromImages(ctx, userID, [][]byte{imageBytes}, []string{filename})
}

func (uc *EnrollmentUseCase) CreateJobFromImages(ctx context.Context, userID string, imagesData [][]byte, filenames []string) (*entity.EnrollmentJob, error) {
	jobID := uuid.NewString()
	var savedPaths []string

	for i, imgBytes := range imagesData {
		ext := ".jpg"
		if i < len(filenames) {
			if e := filepath.Ext(filenames[i]); e != "" {
				ext = e
			}
		}
		fileName := fmt.Sprintf("%s_%d%s", jobID, i, ext)
		if len(imagesData) == 1 {
			fileName = fmt.Sprintf("%s%s", jobID, ext)
		}
		savedPath := filepath.Join(uc.uploadDir, fileName)
		if err := os.WriteFile(savedPath, imgBytes, 0644); err != nil {
			return nil, err
		}
		savedPaths = append(savedPaths, savedPath)
	}

	now := time.Now().UTC()
	expiresAt := now.Add(24 * time.Hour)

	job := &entity.EnrollmentJob{
		ID:        jobID,
		UserID:    userID,
		ImagePath: strings.Join(savedPaths, ","),
		Status:    "pending",
		CreatedAt: now,
		ExpiresAt: &expiresAt,
	}

	if err := uc.enrollmentRepo.CreateJob(ctx, job); err != nil {
		return nil, err
	}

	return job, nil
}

type ClaimJobResponse struct {
	Job       *entity.EnrollmentJob `json:"job,omitempty"`
	ImageB64  string                `json:"image_b64,omitempty"`
	ImagesB64 []string              `json:"images_b64,omitempty"`
}

func (uc *EnrollmentUseCase) ClaimPendingJob(ctx context.Context, deviceID string) (*ClaimJobResponse, error) {
	job, err := uc.enrollmentRepo.ClaimPendingJob(ctx, deviceID)
	if err != nil || job == nil {
		return &ClaimJobResponse{Job: nil}, nil
	}

	var imagesB64 []string
	if job.ImagePath != "" {
		paths := strings.Split(job.ImagePath, ",")
		for _, p := range paths {
			p = strings.TrimSpace(p)
			if p == "" {
				continue
			}
			if b, err := os.ReadFile(p); err == nil {
				imagesB64 = append(imagesB64, base64.StdEncoding.EncodeToString(b))
			}
		}
	}

	var primaryImgB64 string
	if len(imagesB64) > 0 {
		primaryImgB64 = imagesB64[0]
	}

	job.ImageB64 = primaryImgB64
	job.ImagesB64 = imagesB64

	return &ClaimJobResponse{
		Job:       job,
		ImageB64:  primaryImgB64,
		ImagesB64: imagesB64,
	}, nil
}

func (uc *EnrollmentUseCase) SubmitJobResult(ctx context.Context, jobID, deviceID, embeddingB64 string, quality float64, imagesCount ...int) error {
	job, err := uc.enrollmentRepo.GetJobByID(ctx, jobID)
	if err != nil || job == nil {
		return entity.ErrNotFound
	}

	rawEmb, err := base64.StdEncoding.DecodeString(embeddingB64)
	if err != nil || len(rawEmb) == 0 {
		_ = uc.enrollmentRepo.UpdateJobStatus(ctx, jobID, "failed")
		return entity.ErrInvalidInput
	}

	emb := &entity.UserEmbedding{
		ID:           uuid.NewString(),
		UserID:       job.UserID,
		Embedding:    rawEmb,
		ModelVersion: "arcface_mobilefacenet_v1",
		Source:       "kiosk",
		DeviceID:     deviceID,
		QualityScore: quality,
		CreatedAt:    time.Now().UTC(),
	}

	_ = uc.userRepo.SaveEmbedding(ctx, emb)
	_ = uc.enrollmentRepo.UpdateJobStatus(ctx, jobID, "done")

	// PDPA: Clean up temporary enrollment image files
	if job.ImagePath != "" {
		for _, p := range strings.Split(job.ImagePath, ",") {
			_ = os.Remove(strings.TrimSpace(p))
		}
	}

	// Bump user version
	user, err := uc.userRepo.GetByID(ctx, job.UserID)
	if err == nil && user != nil {
		latestV, _ := uc.userRepo.GetLatestVersion(ctx)
		user.Version = latestV + 1
		user.UpdatedAt = time.Now().UTC()
		_ = uc.userRepo.Update(ctx, user)
	}

	// Record Audit Log
	if uc.auditRepo != nil {
		empInfo := job.UserID
		if user != nil {
			if user.EmployeeID != "" {
				empInfo = fmt.Sprintf("[%s] %s", user.EmployeeID, user.Name)
			} else {
				empInfo = user.Name
			}
		}
		deviceName := deviceID
		if uc.deviceRepo != nil {
			if dev, err := uc.deviceRepo.GetByID(ctx, deviceID); err == nil && dev != nil {
				deviceName = dev.Name
			}
		}
		detailMsg := fmt.Sprintf("ลงทะเบียนใบหน้าพนักงาน: %s สำเร็จ (ตู้: %s)", empInfo, deviceName)
		_ = uc.auditRepo.Log(ctx, &entity.AuditLog{
			Action:     "ลงทะเบียนใบหน้า",
			TargetType: "user",
			TargetID:   job.UserID,
			Detail:     detailMsg,
			CreatedAt:  time.Now().UTC(),
		})
	}

	return nil
}
