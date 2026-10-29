package usecase

import (
	"context"

	"attendance-backend/internal/core/domain"
	"attendance-backend/internal/core/ports"
)

type AuditUseCase struct {
	auditRepo ports.AuditRepository
}

func NewAuditUseCase(auditRepo ports.AuditRepository) *AuditUseCase {
	return &AuditUseCase{
		auditRepo: auditRepo,
	}
}

func (uc *AuditUseCase) List(ctx context.Context, limit int) ([]*domain.AuditLog, error) {
	if limit <= 0 {
		limit = 100
	}
	return uc.auditRepo.List(ctx, limit)
}
