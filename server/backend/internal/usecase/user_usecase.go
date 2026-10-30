package usecase

import (
	"context"
	"fmt"
	"strings"
	"time"

	"attendance-backend/internal/entity"
	"attendance-backend/internal/ports"
	"github.com/google/uuid"
)

type UserUseCase struct {
	userRepo  ports.UserRepository
	auditRepo ports.AuditRepository
}

func NewUserUseCase(userRepo ports.UserRepository, auditRepo ports.AuditRepository) *UserUseCase {
	return &UserUseCase{
		userRepo:  userRepo,
		auditRepo: auditRepo,
	}
}

type UserDTO struct {
	EmployeeID string  `json:"employee_id"`
	Title      string  `json:"title"`
	FirstName  string  `json:"first_name"`
	LastName   string  `json:"last_name"`
	Name       string  `json:"name"`
	Department string  `json:"department"`
	Status     string  `json:"status"`
	StartDate  *string `json:"start_date"`
	EndDate    *string `json:"end_date"`
}

func (uc *UserUseCase) List(ctx context.Context, unenrolledOnly bool) ([]*entity.User, error) {
	return uc.userRepo.List(ctx, unenrolledOnly)
}

func (uc *UserUseCase) GetByID(ctx context.Context, id string) (*entity.User, error) {
	return uc.userRepo.GetByID(ctx, id)
}

func (uc *UserUseCase) Create(ctx context.Context, adminID string, dto *UserDTO) (*entity.User, error) {
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
	if dto.Name == "" {
		return nil, fmt.Errorf("%w: user name is required", entity.ErrInvalidInput)
	}

	if dto.Status == "" {
		dto.Status = "active"
	}

	now := time.Now().UTC()

	user := &entity.User{
		ID:         uuid.NewString(),
		EmployeeID: dto.EmployeeID,
		Title:      dto.Title,
		FirstName:  dto.FirstName,
		LastName:   dto.LastName,
		Name:       dto.Name,
		Department: dto.Department,
		Status:     dto.Status,
		StartDate:  dto.StartDate,
		EndDate:    dto.EndDate,
		CreatedAt:  now,
		UpdatedAt:  now,
	}

	if err := uc.userRepo.Create(ctx, user); err != nil {
		return nil, err
	}

	_ = uc.auditRepo.Log(ctx, &entity.AuditLog{
		AdminID:    &adminID,
		Action:     "user.create",
		TargetType: "user",
		TargetID:   user.ID,
		Detail:     fmt.Sprintf(`{"name":"%s","employee_id":"%s"}`, user.Name, user.EmployeeID),
		CreatedAt:  now,
	})

	return user, nil
}

func (uc *UserUseCase) Update(ctx context.Context, adminID, id string, dto *UserDTO) (*entity.User, error) {
	user, err := uc.userRepo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	if dto.EmployeeID != "" {
		user.EmployeeID = dto.EmployeeID
	}
	if dto.Title != "" {
		user.Title = dto.Title
	}
	if dto.FirstName != "" {
		user.FirstName = dto.FirstName
	}
	if dto.LastName != "" {
		user.LastName = dto.LastName
	}
	if dto.Name != "" {
		user.Name = dto.Name
	} else if dto.FirstName != "" || dto.LastName != "" {
		parts := []string{}
		if user.Title != "" {
			parts = append(parts, user.Title)
		}
		if user.FirstName != "" {
			parts = append(parts, user.FirstName)
		}
		if user.LastName != "" {
			parts = append(parts, user.LastName)
		}
		user.Name = strings.TrimSpace(strings.Join(parts, " "))
	}
	if dto.Department != "" {
		user.Department = dto.Department
	}
	if dto.Status != "" {
		user.Status = dto.Status
	}
	if dto.StartDate != nil {
		user.StartDate = dto.StartDate
	}
	if dto.EndDate != nil {
		user.EndDate = dto.EndDate
	}

	user.UpdatedAt = time.Now().UTC()

	if err := uc.userRepo.Update(ctx, user); err != nil {
		return nil, err
	}

	_ = uc.auditRepo.Log(ctx, &entity.AuditLog{
		AdminID:    &adminID,
		Action:     "user.update",
		TargetType: "user",
		TargetID:   user.ID,
		Detail:     fmt.Sprintf(`{"name":"%s","version":%d}`, user.Name, user.Version),
		CreatedAt:  user.UpdatedAt,
	})

	return user, nil
}

func (uc *UserUseCase) Delete(ctx context.Context, adminID, id string) error {
	now := time.Now().UTC()
	if err := uc.userRepo.Delete(ctx, id); err != nil {
		return err
	}

	_ = uc.auditRepo.Log(ctx, &entity.AuditLog{
		AdminID:    &adminID,
		Action:     "user.delete",
		TargetType: "user",
		TargetID:   id,
		CreatedAt:  now,
	})
	return nil
}

func (uc *UserUseCase) SaveEmbedding(ctx context.Context, adminID, userID string, embedding []byte, quality float64) error {
	emb := &entity.UserEmbedding{
		ID:           uuid.NewString(),
		UserID:       userID,
		Embedding:    embedding,
		ModelVersion: "arcface_mobilefacenet_v1",
		Source:       "dashboard",
		QualityScore: quality,
		CreatedAt:    time.Now().UTC(),
	}

	if err := uc.userRepo.SaveEmbedding(ctx, emb); err != nil {
		return err
	}

	// Bump user version
	user, err := uc.userRepo.GetByID(ctx, userID)
	if err == nil && user != nil {
		latestV, _ := uc.userRepo.GetLatestVersion(ctx)
		user.Version = latestV + 1
		user.UpdatedAt = time.Now().UTC()
		_ = uc.userRepo.Update(ctx, user)
	}

	return nil
}
