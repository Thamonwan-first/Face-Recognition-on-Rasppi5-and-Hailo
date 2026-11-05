package usecase

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"attendance-backend/internal/entity"
	"attendance-backend/internal/ports"
)

type SettingsUseCase struct {
	settingsRepo ports.SettingsRepository
	auditRepo    ports.AuditRepository
}

func NewSettingsUseCase(settingsRepo ports.SettingsRepository, auditRepo ...ports.AuditRepository) *SettingsUseCase {
	var aRepo ports.AuditRepository
	if len(auditRepo) > 0 {
		aRepo = auditRepo[0]
	}
	return &SettingsUseCase{
		settingsRepo: settingsRepo,
		auditRepo:    aRepo,
	}
}

func (uc *SettingsUseCase) Get(ctx context.Context) (*entity.SystemSettings, error) {
	return uc.settingsRepo.GetSettings(ctx)
}

func (uc *SettingsUseCase) Update(ctx context.Context, adminID string, s *entity.SystemSettings) error {
	if s.WorkStartTime == "" {
		return errors.New("work_start_time is required (e.g. 08:30)")
	}
	if s.WorkEndTime == "" {
		return errors.New("work_end_time is required (e.g. 17:30)")
	}
	if s.MinWorkHours <= 0 {
		s.MinWorkHours = 8.0
	}
	if s.DebounceSeconds <= 0 {
		s.DebounceSeconds = 60
	}
	if s.RecognitionThreshold <= 0 || s.RecognitionThreshold >= 1.0 {
		s.RecognitionThreshold = 0.65
	}

	if err := uc.settingsRepo.UpdateSettings(ctx, s); err != nil {
		return err
	}

	if uc.auditRepo != nil {
		detailBytes, _ := json.Marshal(s)
		var aID *string
		if adminID != "" {
			aID = &adminID
		}
		_ = uc.auditRepo.Log(ctx, &entity.AuditLog{
			AdminID:    aID,
			Action:     "settings:update",
			TargetType: "system_settings",
			TargetID:   "global",
			Detail:     string(detailBytes),
			CreatedAt:  time.Now().UTC(),
		})
	}

	return nil
}
