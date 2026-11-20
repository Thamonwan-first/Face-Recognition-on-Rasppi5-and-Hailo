package usecase_test

import (
	"context"
	"testing"
	"time"

	"attendance-backend/internal/entity"
	"attendance-backend/internal/usecase"
)

func TestDeviceUseCase_Update(t *testing.T) {
	ctx := context.Background()

	initialDevice := &entity.Device{
		ID:        "kiosk-01",
		Name:      "เครื่องหน้าประตู",
		Location:  "อาคาร 1 ชั้น 1",
		Direction: "AUTO",
		Mode:      "attendance",
		CreatedAt: time.Now().UTC(),
	}

	devRepo := &mockDeviceRepo{
		devices: []*entity.Device{initialDevice},
	}

	deviceUC := usecase.NewDeviceUseCase(devRepo, nil)

	t.Run("success update with all valid fields", func(t *testing.T) {
		dto := &usecase.UpdateDeviceDTO{
			Name:      "ตู้หน้าห้องประชุม 1",
			Location:  "อาคาร 2 ชั้น 3",
			Direction: "in",
			Mode:      "meeting",
		}
		updated, err := deviceUC.Update(ctx, "admin-01", "kiosk-01", dto)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if updated.Name != "ตู้หน้าห้องประชุม 1" {
			t.Errorf("expected name 'ตู้หน้าห้องประชุม 1', got '%s'", updated.Name)
		}
		if updated.Location != "อาคาร 2 ชั้น 3" {
			t.Errorf("expected location 'อาคาร 2 ชั้น 3', got '%s'", updated.Location)
		}
		if updated.Direction != "IN" {
			t.Errorf("expected direction normalized to 'IN', got '%s'", updated.Direction)
		}
		if updated.Mode != "meeting" {
			t.Errorf("expected mode 'meeting', got '%s'", updated.Mode)
		}
	})

	t.Run("clear location with empty string", func(t *testing.T) {
		dto := &usecase.UpdateDeviceDTO{
			Name:     "ตู้หน้าห้องประชุม 1",
			Location: "",
		}
		updated, err := deviceUC.Update(ctx, "admin-01", "kiosk-01", dto)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if updated.Location != "" {
			t.Errorf("expected location to be cleared, got '%s'", updated.Location)
		}
	})

	t.Run("invalid direction is ignored", func(t *testing.T) {
		dto := &usecase.UpdateDeviceDTO{
			Direction: "INVALID_DIR",
		}
		updated, err := deviceUC.Update(ctx, "admin-01", "kiosk-01", dto)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if updated.Direction != "IN" { // retained previous valid direction
			t.Errorf("expected direction to remain 'IN', got '%s'", updated.Direction)
		}
	})

	t.Run("invalid mode falls back to attendance", func(t *testing.T) {
		dto := &usecase.UpdateDeviceDTO{
			Mode: "unknown_mode",
		}
		updated, err := deviceUC.Update(ctx, "admin-01", "kiosk-01", dto)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if updated.Mode != "attendance" {
			t.Errorf("expected mode fallback to 'attendance', got '%s'", updated.Mode)
		}
	})

	t.Run("device not found returns error", func(t *testing.T) {
		dto := &usecase.UpdateDeviceDTO{Name: "Non-existent"}
		_, err := deviceUC.Update(ctx, "admin-01", "unknown-device", dto)
		if err == nil {
			t.Error("expected error for non-existent device, got nil")
		}
	})
}
