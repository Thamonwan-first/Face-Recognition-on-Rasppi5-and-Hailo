package usecase

import (
	"context"
	"time"

	"attendance-backend/internal/core/domain"
	"attendance-backend/internal/core/ports"
	"attendance-backend/internal/pkg/hasher"
	"attendance-backend/internal/pkg/token"
)

type AuthUseCase struct {
	adminRepo  ports.AdminRepository
	auditRepo  ports.AuditRepository
	tokenMaker *token.TokenMaker
}

func NewAuthUseCase(adminRepo ports.AdminRepository, auditRepo ports.AuditRepository, tokenMaker *token.TokenMaker) *AuthUseCase {
	return &AuthUseCase{
		adminRepo:  adminRepo,
		auditRepo:  auditRepo,
		tokenMaker: tokenMaker,
	}
}

type LoginResponse struct {
	AccessToken string        `json:"access_token"`
	Token       string        `json:"token"`
	TokenType   string        `json:"token_type"`
	User        *domain.Admin `json:"user"`
	Admin       *domain.Admin `json:"admin"`
}

func (uc *AuthUseCase) Login(ctx context.Context, username, password string) (*LoginResponse, error) {
	admin, err := uc.adminRepo.GetByUsername(ctx, username)
	if err != nil {
		return nil, domain.ErrUnauthorized
	}

	if !hasher.CheckPassword(password, admin.PasswordHash) {
		return nil, domain.ErrUnauthorized
	}

	now := time.Now().UTC()
	_ = uc.adminRepo.UpdateLastLogin(ctx, admin.ID, now)
	admin.LastLogin = &now

	accessToken, err := uc.tokenMaker.CreateToken(admin.ID, admin.Username, admin.Role, 24*time.Hour)
	if err != nil {
		return nil, domain.ErrInternalServer
	}

	_ = uc.auditRepo.Log(ctx, &domain.AuditLog{
		AdminID:    &admin.ID,
		Action:     "admin.login",
		TargetType: "admin",
		TargetID:   admin.ID,
		CreatedAt:  now,
	})

	return &LoginResponse{
		AccessToken: accessToken,
		Token:       accessToken,
		TokenType:   "bearer",
		User:        admin,
		Admin:       admin,
	}, nil
}

func (uc *AuthUseCase) GetCurrentAdmin(ctx context.Context, adminID string) (*domain.Admin, error) {
	return uc.adminRepo.GetByID(ctx, adminID)
}
