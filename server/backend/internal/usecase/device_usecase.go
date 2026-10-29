package usecase

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"strconv"
	"time"

	"attendance-backend/internal/core/domain"
	"attendance-backend/internal/core/ports"
	"attendance-backend/internal/pkg/hasher"
)

type DeviceUseCase struct {
	deviceRepo ports.DeviceRepository
	auditRepo  ports.AuditRepository
}

func NewDeviceUseCase(deviceRepo ports.DeviceRepository, auditRepo ports.AuditRepository) *DeviceUseCase {
	return &DeviceUseCase{
		deviceRepo: deviceRepo,
		auditRepo:  auditRepo,
	}
}

type DeviceDTO struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Location  string `json:"location"`
	Direction string `json:"direction"`
}

type RegisterDeviceResponse struct {
	Device *domain.Device `json:"device"`
	APIKey string         `json:"api_key"`
}

func (uc *DeviceUseCase) List(ctx context.Context) ([]*domain.Device, error) {
	devices, err := uc.deviceRepo.List(ctx)
	if err != nil {
		return nil, err
	}

	threshold := 60 * time.Second
	if secStr := os.Getenv("DEVICE_OFFLINE_THRESHOLD_SECONDS"); secStr != "" {
		if sec, err := strconv.Atoi(secStr); err == nil && sec > 0 {
			threshold = time.Duration(sec) * time.Second
		}
	}

	now := time.Now().UTC()
	for _, d := range devices {
		if d.RevokedAt != nil {
			d.Status = "revoked"
		} else if d.LastSeen != nil && now.Sub(d.LastSeen.UTC()) <= threshold {
			d.Status = "online"
		} else {
			d.Status = "offline"
		}

		if d.State != nil {
			d.CPUTemp = d.State.CPUTemp
			d.FPS = d.State.FPS
			d.OutboxPending = d.State.OutboxPending
		}
	}

	return devices, nil
}

func (uc *DeviceUseCase) Register(ctx context.Context, adminID string, dto *DeviceDTO) (*RegisterDeviceResponse, error) {
	if dto.ID == "" {
		return nil, fmt.Errorf("%w: device id is required", domain.ErrInvalidInput)
	}
	if dto.Name == "" {
		dto.Name = dto.ID
	}
	if dto.Direction == "" {
		dto.Direction = "AUTO"
	}

	rawBytes := make([]byte, 32)
	_, _ = rand.Read(rawBytes)
	apiKey := hex.EncodeToString(rawBytes)

	hash := hasher.HashAPIKey(apiKey)

	now := time.Now().UTC()
	device := &domain.Device{
		ID:         dto.ID,
		Name:       dto.Name,
		Location:   dto.Location,
		Direction:  dto.Direction,
		ApiKeyHash: hash,
		Status:     "offline",
		CreatedAt:  now,
	}

	if err := uc.deviceRepo.Create(ctx, device); err != nil {
		return nil, err
	}

	_ = uc.auditRepo.Log(ctx, &domain.AuditLog{
		AdminID:    &adminID,
		Action:     "device.register",
		TargetType: "device",
		TargetID:   device.ID,
		CreatedAt:  now,
	})

	return &RegisterDeviceResponse{
		Device: device,
		APIKey: apiKey,
	}, nil
}

func (uc *DeviceUseCase) AuthenticateDevice(ctx context.Context, apiKey string) (*domain.Device, error) {
	if apiKey == "" {
		return nil, domain.ErrUnauthorized
	}

	hash := hasher.HashAPIKey(apiKey)
	device, err := uc.deviceRepo.GetByApiKeyHash(ctx, hash)
	if err == nil && device != nil {
		return device, nil
	}

	// Backward compatibility fallback for legacy bcrypt keys
	devices, err := uc.deviceRepo.List(ctx)
	if err != nil {
		return nil, domain.ErrUnauthorized
	}

	for _, dev := range devices {
		if dev.Status != "revoked" && hasher.CheckPassword(apiKey, dev.ApiKeyHash) {
			return dev, nil
		}
	}
	return nil, domain.ErrUnauthorized
}

func (uc *DeviceUseCase) Delete(ctx context.Context, adminID, id string) error {
	now := time.Now().UTC()
	if err := uc.deviceRepo.Delete(ctx, id); err != nil {
		return err
	}

	_ = uc.auditRepo.Log(ctx, &domain.AuditLog{
		AdminID:    &adminID,
		Action:     "device.delete",
		TargetType: "device",
		TargetID:   id,
		CreatedAt:  now,
	})
	return nil
}
