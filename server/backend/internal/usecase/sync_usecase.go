package usecase

import (
	"context"
	"encoding/base64"
	"strings"
	"time"

	"attendance-backend/internal/entity"
	"attendance-backend/internal/ports"
	"github.com/google/uuid"
)

type SyncUseCase struct {
	deviceRepo     ports.DeviceRepository
	userRepo       ports.UserRepository
	attendanceUC   *AttendanceUseCase
	enrollmentRepo ports.EnrollmentRepository
}

func NewSyncUseCase(
	deviceRepo ports.DeviceRepository,
	userRepo ports.UserRepository,
	attendanceUC *AttendanceUseCase,
	enrollmentRepo ports.EnrollmentRepository,
) *SyncUseCase {
	return &SyncUseCase{
		deviceRepo:     deviceRepo,
		userRepo:       userRepo,
		attendanceUC:   attendanceUC,
		enrollmentRepo: enrollmentRepo,
	}
}

type HeartbeatDTO struct {
	AppliedVersion int      `json:"applied_version"`
	SwVersion      string   `json:"sw_version"`
	ModelVersion   string   `json:"model_version"`
	CPUTemp        *float64 `json:"cpu_temp"`
	FPS            *float64 `json:"fps"`
	OutboxPending  int      `json:"outbox_pending"`
	IPAddress      string   `json:"ip_address"`
}

func (uc *SyncUseCase) Heartbeat(ctx context.Context, deviceID string, dto *HeartbeatDTO) error {
	now := time.Now().UTC()
	_ = uc.deviceRepo.UpdateStatusAndIP(ctx, deviceID, "online", dto.IPAddress, now)

	state := &entity.DeviceState{
		DeviceID:       deviceID,
		AppliedVersion: dto.AppliedVersion,
		SwVersion:      dto.SwVersion,
		ModelVersion:   dto.ModelVersion,
		CPUTemp:        dto.CPUTemp,
		FPS:            dto.FPS,
		OutboxPending:  dto.OutboxPending,
		UpdatedAt:      now,
	}
	return uc.deviceRepo.UpdateState(ctx, state)
}

type UserSyncItem struct {
	ID           string  `json:"id"`
	EmployeeID   string  `json:"employee_id"`
	Title        string  `json:"title"`
	FirstName    string  `json:"first_name"`
	LastName     string  `json:"last_name"`
	Name         string  `json:"name"`
	Department   string  `json:"department"`
	Status       string  `json:"status"`
	Version      int     `json:"version"`
	PhotoURL     *string `json:"photo_url,omitempty"`
	EmbeddingB64 *string `json:"embedding_b64"`
}

type UserSyncResponse struct {
	Users         []*UserSyncItem `json:"users"`
	LatestVersion int             `json:"latest_version"`
}

func (uc *SyncUseCase) GetUsersSince(ctx context.Context, sinceVersion int) (*UserSyncResponse, error) {
	users, err := uc.userRepo.ListUpdatedSince(ctx, sinceVersion)
	if err != nil {
		return nil, err
	}

	latestV, _ := uc.userRepo.GetLatestVersion(ctx)

	items := make([]*UserSyncItem, 0, len(users))
	for _, u := range users {
		item := &UserSyncItem{
			ID:         u.ID,
			EmployeeID: u.EmployeeID,
			Title:      u.Title,
			FirstName:  u.FirstName,
			LastName:   u.LastName,
			Name:       u.Name,
			Department: u.Department,
			Status:     u.Status,
			Version:    u.Version,
		}
		if u.PhotoURL != "" {
			item.PhotoURL = &u.PhotoURL
		}

		// Only include embedding vector if the user is currently active
		if u.Status == "active" && len(u.RawEmbedding) > 0 {
			b64 := base64.StdEncoding.EncodeToString(u.RawEmbedding)
			item.EmbeddingB64 = &b64
		}
		items = append(items, item)
	}

	return &UserSyncResponse{
		Users:         items,
		LatestVersion: latestV,
	}, nil
}

type KioskEnrollmentDTO struct {
	UserID       string  `json:"user_id"`
	EmployeeID   string  `json:"employee_id"`
	Title        string  `json:"title"`
	FirstName    string  `json:"first_name"`
	LastName     string  `json:"last_name"`
	Name         string  `json:"name"`
	Department   string  `json:"department"`
	EmbeddingB64 string  `json:"embedding_b64"`
	QualityScore float64 `json:"quality_score"`
}

func (uc *SyncUseCase) SaveKioskEnrollment(ctx context.Context, deviceID string, dto *KioskEnrollmentDTO) error {
	now := time.Now().UTC()
	latestV, _ := uc.userRepo.GetLatestVersion(ctx)
	newVersion := latestV + 1

	if dto.Name == "" {
		parts := []string{}
		if dto.Title != "" {
			parts = append(parts, dto.Title)
		}
		if dto.FirstName != "" {
			parts = append(parts, dto.FirstName)
		}
		if dto.LastName != "" {
			parts = append(parts, dto.LastName)
		}
		dto.Name = strings.TrimSpace(strings.Join(parts, " "))
	}

	user, err := uc.userRepo.GetByID(ctx, dto.UserID)
	if err != nil || user == nil {
		user = &entity.User{
			ID:         dto.UserID,
			EmployeeID: dto.EmployeeID,
			Title:      dto.Title,
			FirstName:  dto.FirstName,
			LastName:   dto.LastName,
			Name:       dto.Name,
			Department: dto.Department,
			Status:     "active",
			Version:    newVersion,
			CreatedAt:  now,
			UpdatedAt:  now,
		}
		if err := uc.userRepo.Create(ctx, user); err != nil {
			return err
		}
	} else {
		user.Name = dto.Name
		if dto.EmployeeID != "" {
			user.EmployeeID = dto.EmployeeID
		}
		user.Version = newVersion
		user.UpdatedAt = now
		if err := uc.userRepo.Update(ctx, user); err != nil {
			return err
		}
	}

	if dto.EmbeddingB64 != "" {
		rawEmb, err := base64.StdEncoding.DecodeString(dto.EmbeddingB64)
		if err != nil {
			return err
		}
		if len(rawEmb) > 0 {
			emb := &entity.UserEmbedding{
				ID:           uuid.NewString(),
				UserID:       user.ID,
				Embedding:    rawEmb,
				ModelVersion: "arcface_mobilefacenet_v1",
				Source:       "kiosk",
				DeviceID:     deviceID,
				QualityScore: dto.QualityScore,
				CreatedAt:    now,
			}
			if err := uc.userRepo.SaveEmbedding(ctx, emb); err != nil {
				return err
			}
		}
	}

	return nil
}

func (uc *SyncUseCase) ProcessAttendanceEvents(ctx context.Context, deviceID string, events []*entity.AttendanceEvent) ([]string, error) {
	syncedIDs := make([]string, 0, len(events))
	for _, ev := range events {
		ev.DeviceID = deviceID
		if err := uc.attendanceUC.ProcessEvent(ctx, ev); err == nil {
			syncedIDs = append(syncedIDs, ev.EventID)
		}
	}
	return syncedIDs, nil
}

func (uc *SyncUseCase) ProcessUnrecognizedScans(ctx context.Context, deviceID string, scans []*entity.UnrecognizedScan) (int, error) {
	count := 0
	for _, s := range scans {
		s.DeviceID = deviceID
		if err := uc.attendanceUC.SaveUnrecognized(ctx, s); err == nil {
			count++
		}
	}
	return count, nil
}
