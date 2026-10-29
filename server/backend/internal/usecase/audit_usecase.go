package usecase

import (
	"context"

	"attendance-backend/internal/entity"
	"attendance-backend/internal/ports"
)

type AuditUseCase struct {
	auditRepo ports.AuditRepository
}

func NewAuditUseCase(auditRepo ports.AuditRepository) *AuditUseCase {
	return &AuditUseCase{
		auditRepo: auditRepo,
	}
}

func (uc *AuditUseCase) List(ctx context.Context, limit int) ([]*entity.AuditLog, error) {
	if limit <= 0 {
		limit = 100
	}
	return uc.auditRepo.List(ctx, limit)
}
