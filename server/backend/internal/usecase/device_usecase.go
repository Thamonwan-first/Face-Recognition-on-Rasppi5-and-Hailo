package usecase

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"attendance-backend/internal/entity"
	"attendance-backend/internal/ports"
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
	IPAddress string `json:"ip_address"`
}

type RegisterDeviceResponse struct {
	Device *entity.Device `json:"device"`
	APIKey string         `json:"api_key"`
}

func (uc *DeviceUseCase) List(ctx context.Context) ([]*entity.Device, error) {
	devices, err := uc.deviceRepo.List(ctx)
	if err != nil {
		return nil, err
	}

	threshold := 10 * time.Second
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
		return nil, fmt.Errorf("%w: device id is required", entity.ErrInvalidInput)
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
	device := &entity.Device{
		ID:         dto.ID,
		Name:       dto.Name,
		Location:   dto.Location,
		Direction:  dto.Direction,
		IPAddress:  dto.IPAddress,
		ApiKeyHash: hash,
		Status:     "offline",
		CreatedAt:  now,
	}

	if err := uc.deviceRepo.Create(ctx, device); err != nil {
		return nil, err
	}

	_ = uc.auditRepo.Log(ctx, &entity.AuditLog{
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

func (uc *DeviceUseCase) AuthenticateDevice(ctx context.Context, apiKey string) (*entity.Device, error) {
	if apiKey == "" {
		return nil, entity.ErrUnauthorized
	}

	hash := hasher.HashAPIKey(apiKey)
	device, err := uc.deviceRepo.GetByApiKeyHash(ctx, hash)
	if err == nil && device != nil {
		return device, nil
	}

	// Backward compatibility fallback: only test devices that actually contain a legacy bcrypt hash ($2a$/$2b$)
	devices, err := uc.deviceRepo.List(ctx)
	if err != nil {
		return nil, entity.ErrUnauthorized
	}

	for _, dev := range devices {
		if dev.Status != "revoked" && (strings.HasPrefix(dev.ApiKeyHash, "$2a$") || strings.HasPrefix(dev.ApiKeyHash, "$2b$")) {
			if hasher.CheckPassword(apiKey, dev.ApiKeyHash) {
				return dev, nil
			}
		}
	}
	return nil, entity.ErrUnauthorized
}

func (uc *DeviceUseCase) Delete(ctx context.Context, adminID, id string) error {
	now := time.Now().UTC()
	if err := uc.deviceRepo.Delete(ctx, id); err != nil {
		return err
	}

	_ = uc.auditRepo.Log(ctx, &entity.AuditLog{
		AdminID:    &adminID,
		Action:     "device.delete",
		TargetType: "device",
		TargetID:   id,
		CreatedAt:  now,
	})
	return nil
}

type UpdateDeviceDTO struct {
	Name      string `json:"name"`
	Location  string `json:"location"`
	Direction string `json:"direction"`
	Mode      string `json:"mode"`
}

func (uc *DeviceUseCase) Update(ctx context.Context, adminID, id string, dto *UpdateDeviceDTO) (*entity.Device, error) {
	device, err := uc.deviceRepo.GetByID(ctx, id)
	if err != nil || device == nil {
		return nil, fmt.Errorf("device not found: %s", id)
	}

	trimmedName := strings.TrimSpace(dto.Name)
	if trimmedName != "" {
		device.Name = trimmedName
	}
	device.Location = strings.TrimSpace(dto.Location)

	if dto.Direction != "" {
		dir := strings.ToUpper(strings.TrimSpace(dto.Direction))
		if dir == "IN" || dir == "OUT" || dir == "AUTO" {
			device.Direction = dir
		}
	}

	if dto.Mode != "" {
		mode := strings.ToLower(strings.TrimSpace(dto.Mode))
		if mode != "attendance" && mode != "meeting" {
			mode = "attendance"
		}
		device.Mode = mode
	}

	if err := uc.deviceRepo.Update(ctx, device); err != nil {
		return nil, err
	}

	now := time.Now().UTC()
	if uc.auditRepo != nil {
		_ = uc.auditRepo.Log(ctx, &entity.AuditLog{
			AdminID:    &adminID,
			Action:     "device.update",
			TargetType: "device",
			TargetID:   id,
			Detail:     fmt.Sprintf("แก้ไขข้อมูลอุปกรณ์: %s (โหมด: %s)", device.Name, device.Mode),
			CreatedAt:  now,
		})
	}

	return device, nil
}

func (uc *DeviceUseCase) UpdateIP(ctx context.Context, id, ip string) error {
	if id == "" || ip == "" {
		return nil
	}
	return uc.deviceRepo.UpdateIPAddress(ctx, id, ip)
}

func (uc *DeviceUseCase) UpdateMode(ctx context.Context, adminID, id, mode string) error {
	if mode != "attendance" && mode != "meeting" {
		mode = "attendance"
	}
	if err := uc.deviceRepo.UpdateMode(ctx, id, mode); err != nil {
		return err
	}
	now := time.Now().UTC()
	_ = uc.auditRepo.Log(ctx, &entity.AuditLog{
		AdminID:    &adminID,
		Action:     "device.update_mode",
		TargetType: "device",
		TargetID:   id,
		Detail:     fmt.Sprintf("เปลี่ยนโหมดเครื่องเป็น %s", mode),
		CreatedAt:  now,
	})
	return nil
}
