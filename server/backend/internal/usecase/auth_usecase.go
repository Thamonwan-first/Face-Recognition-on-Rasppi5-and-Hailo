package usecase

import (
	"context"
	"errors"
	"fmt"
	"time"

	"attendance-backend/internal/entity"
	"attendance-backend/internal/pkg/hasher"
	"attendance-backend/internal/pkg/token"
	"attendance-backend/internal/ports"
	"github.com/google/uuid"
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
	User        *entity.Admin `json:"user"`
	Admin       *entity.Admin `json:"admin"`
}

func (uc *AuthUseCase) Login(ctx context.Context, username, password string) (*LoginResponse, error) {
	admin, err := uc.adminRepo.GetByUsername(ctx, username)
	if err != nil {
		return nil, entity.ErrUnauthorized
	}

	if !hasher.CheckPassword(password, admin.PasswordHash) {
		return nil, entity.ErrUnauthorized
	}

	now := time.Now().UTC()
	_ = uc.adminRepo.UpdateLastLogin(ctx, admin.ID, now)
	admin.LastLogin = &now

	accessToken, err := uc.tokenMaker.CreateToken(admin.ID, admin.Username, admin.Role, admin.Permissions, 24*time.Hour)
	if err != nil {
		return nil, entity.ErrInternalServer
	}

	_ = uc.auditRepo.Log(ctx, &entity.AuditLog{
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

func (uc *AuthUseCase) GetCurrentAdmin(ctx context.Context, adminID string) (*entity.Admin, error) {
	return uc.adminRepo.GetByID(ctx, adminID)
}

type CreateAdminDTO struct {
	Username    string   `json:"username" binding:"required"`
	Password    string   `json:"password" binding:"required"`
	Role        string   `json:"role"`
	Permissions []string `json:"permissions"`
}

type UpdateAdminDTO struct {
	Role        string   `json:"role"`
	Permissions []string `json:"permissions"`
}

type ChangePasswordDTO struct {
	OldPassword string `json:"old_password" binding:"required"`
	NewPassword string `json:"new_password" binding:"required"`
}

func (uc *AuthUseCase) ListAdmins(ctx context.Context) ([]*entity.Admin, error) {
	return uc.adminRepo.List(ctx)
}

func (uc *AuthUseCase) CreateAdmin(ctx context.Context, performerID string, dto *CreateAdminDTO) (*entity.Admin, error) {
	if dto.Username == "" || dto.Password == "" {
		return nil, errors.New("username and password are required")
	}

	// Check existing username
	existing, _ := uc.adminRepo.GetByUsername(ctx, dto.Username)
	if existing != nil {
		return nil, errors.New("ชื่อผู้ใช้งานนี้มีอยู่ในระบบแล้ว")
	}

	if len(dto.Password) < 6 {
		return nil, errors.New("รหัสผ่านต้องมีความยาวอย่างน้อย 6 ตัวอักษร")
	}

	hash, err := hasher.HashPassword(dto.Password)
	if err != nil {
		return nil, fmt.Errorf("failed to hash password: %w", err)
	}

	role := dto.Role
	if role == "" {
		role = "viewer"
	}
	perms := dto.Permissions
	if perms == nil {
		perms = []string{}
	}

	now := time.Now().UTC()
	newAdmin := &entity.Admin{
		ID:           uuid.NewString(),
		Username:     dto.Username,
		PasswordHash: hash,
		Role:         role,
		Permissions:  perms,
		CreatedAt:    now,
	}

	if err := uc.adminRepo.Create(ctx, newAdmin); err != nil {
		return nil, err
	}

	_ = uc.auditRepo.Log(ctx, &entity.AuditLog{
		AdminID:    &performerID,
		Action:     "admin.create",
		TargetType: "admin",
		TargetID:   newAdmin.ID,
		Detail:     fmt.Sprintf("สร้างผู้ดูแลระบบ: %s (บทบาท: %s)", newAdmin.Username, newAdmin.Role),
		CreatedAt:  now,
	})

	return newAdmin, nil
}

func (uc *AuthUseCase) UpdateAdmin(ctx context.Context, performerID, id string, dto *UpdateAdminDTO) (*entity.Admin, error) {
	admin, err := uc.adminRepo.GetByID(ctx, id)
	if err != nil {
		return nil, errors.New("ไม่พบบัญชีผู้ใช้นี้ในระบบ")
	}

	if dto.Role != "" {
		admin.Role = dto.Role
	}
	if dto.Permissions != nil {
		admin.Permissions = dto.Permissions
	}

	if err := uc.adminRepo.Update(ctx, admin); err != nil {
		return nil, err
	}

	_ = uc.auditRepo.Log(ctx, &entity.AuditLog{
		AdminID:    &performerID,
		Action:     "admin.update",
		TargetType: "admin",
		TargetID:   admin.ID,
		Detail:     fmt.Sprintf("แก้ไขสิทธิ์ผู้ดูแลระบบ: %s", admin.Username),
		CreatedAt:  time.Now().UTC(),
	})

	return admin, nil
}

func (uc *AuthUseCase) DeleteAdmin(ctx context.Context, performerID, id string) error {
	if performerID == id {
		return errors.New("ไม่สามารถลบบัญชีของตนเองที่กำลังเข้าใช้งานอยู่ได้")
	}

	admin, err := uc.adminRepo.GetByID(ctx, id)
	if err != nil {
		return errors.New("ไม่พบบัญชีผู้ใช้นี้ในระบบ")
	}

	if admin.Username == "admin" {
		return errors.New("ไม่อนุญาตให้ลบบัญชีผู้ดูแลระบบหลัก (admin)")
	}

	if err := uc.adminRepo.Delete(ctx, id); err != nil {
		return err
	}

	_ = uc.auditRepo.Log(ctx, &entity.AuditLog{
		AdminID:    &performerID,
		Action:     "admin.delete",
		TargetType: "admin",
		TargetID:   id,
		Detail:     fmt.Sprintf("ลบบัญชีผู้ดูแลระบบ: %s", admin.Username),
		CreatedAt:  time.Now().UTC(),
	})

	return nil
}

func (uc *AuthUseCase) ChangePassword(ctx context.Context, adminID, oldPassword, newPassword string) error {
	admin, err := uc.adminRepo.GetByID(ctx, adminID)
	if err != nil {
		return errors.New("ไม่พบบัญชีผู้ใช้งาน")
	}

	if !hasher.CheckPassword(oldPassword, admin.PasswordHash) {
		return errors.New("รหัสผ่านเดิมไม่ถูกต้อง")
	}

	if len(newPassword) < 6 {
		return errors.New("รหัสผ่านใหม่ต้องมีความยาวอย่างน้อย 6 ตัวอักษร")
	}

	newHash, err := hasher.HashPassword(newPassword)
	if err != nil {
		return fmt.Errorf("failed to hash new password: %w", err)
	}

	if err := uc.adminRepo.UpdatePassword(ctx, adminID, newHash); err != nil {
		return err
	}

	_ = uc.auditRepo.Log(ctx, &entity.AuditLog{
		AdminID:    &adminID,
		Action:     "admin.change_password",
		TargetType: "admin",
		TargetID:   adminID,
		Detail:     "เปลี่ยนรหัสผ่านสำเร็จ",
		CreatedAt:  time.Now().UTC(),
	})

	return nil
}
