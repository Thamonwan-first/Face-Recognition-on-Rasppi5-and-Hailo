package usecase

import (
	"context"
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"attendance-backend/internal/entity"
	"attendance-backend/internal/ports"
	"github.com/google/uuid"
)

type EnrollmentUseCase struct {
	enrollmentRepo ports.EnrollmentRepository
	userRepo       ports.UserRepository
	uploadDir      string
}

func NewEnrollmentUseCase(enrollmentRepo ports.EnrollmentRepository, userRepo ports.UserRepository, uploadDir string) *EnrollmentUseCase {
	if uploadDir == "" {
		uploadDir = "data/enrollment_images"
	}
	_ = os.MkdirAll(uploadDir, 0755)
	return &EnrollmentUseCase{
		enrollmentRepo: enrollmentRepo,
		userRepo:       userRepo,
		uploadDir:      uploadDir,
	}
}

func (uc *EnrollmentUseCase) GetPendingJobs(ctx context.Context) ([]*entity.EnrollmentJob, error) {
	return uc.enrollmentRepo.GetPendingJobs(ctx)
}

func (uc *EnrollmentUseCase) CreateJobFromImage(ctx context.Context, userID string, imageBytes []byte, filename string) (*entity.EnrollmentJob, error) {
	jobID := uuid.NewString()
	ext := filepath.Ext(filename)
	if ext == "" {
		ext = ".jpg"
	}
	savedPath := filepath.Join(uc.uploadDir, fmt.Sprintf("%s%s", jobID, ext))

	if err := os.WriteFile(savedPath, imageBytes, 0644); err != nil {
		return nil, err
	}

	now := time.Now().UTC()
	expiresAt := now.Add(24 * time.Hour)

	job := &entity.EnrollmentJob{
		ID:        jobID,
		UserID:    userID,
		ImagePath: savedPath,
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

	var imgB64 string
	if job.ImagePath != "" {
		if b, err := os.ReadFile(job.ImagePath); err == nil {
			imgB64 = base64.StdEncoding.EncodeToString(b)
		}
	}

	return &ClaimJobResponse{
		Job:       job,
		ImageB64:  imgB64,
		ImagesB64: []string{imgB64},
	}, nil
}

func (uc *EnrollmentUseCase) SubmitJobResult(ctx context.Context, jobID, deviceID, embeddingB64 string, quality float64) error {
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

	// Bump user version
	user, err := uc.userRepo.GetByID(ctx, job.UserID)
	if err == nil && user != nil {
		latestV, _ := uc.userRepo.GetLatestVersion(ctx)
		user.Version = latestV + 1
		user.UpdatedAt = time.Now().UTC()
		_ = uc.userRepo.Update(ctx, user)
	}

	return nil
}
