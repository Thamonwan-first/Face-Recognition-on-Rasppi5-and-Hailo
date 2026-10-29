package usecase

import (
	"context"
	"time"

	"attendance-backend/internal/core/domain"
	"attendance-backend/internal/core/ports"
)

type AttendanceUseCase struct {
	attendanceRepo ports.AttendanceRepository
}

func NewAttendanceUseCase(attendanceRepo ports.AttendanceRepository) *AttendanceUseCase {
	return &AttendanceUseCase{
		attendanceRepo: attendanceRepo,
	}
}

func (uc *AttendanceUseCase) ListPairs(ctx context.Context, startDate, endDate, userID, department string, limit int) ([]*domain.AttendancePair, error) {
	if limit <= 0 {
		limit = 300
	}
	return uc.attendanceRepo.ListPairs(ctx, startDate, endDate, userID, department, limit)
}

func (uc *AttendanceUseCase) ListEvents(ctx context.Context, limit int) ([]*domain.AttendanceEvent, error) {
	if limit <= 0 {
		limit = 100
	}
	return uc.attendanceRepo.ListEvents(ctx, limit)
}

// ProcessEvent processes an incoming attendance event and updates/creates attendance pairs
func (uc *AttendanceUseCase) ProcessEvent(ctx context.Context, event *domain.AttendanceEvent) error {
	// 1. Check idempotency
	existing, err := uc.attendanceRepo.GetEventByID(ctx, event.EventID)
	if err == nil && existing != nil {
		return nil // already processed
	}

	if event.SyncedAt.IsZero() {
		event.SyncedAt = time.Now().UTC()
	}

	// 2. Save event
	if err := uc.attendanceRepo.SaveEvent(ctx, event); err != nil {
		return err
	}

	// 3. Date string YYYY-MM-DD
	workDate := event.CreatedAt.Format("2006-01-02")

	// 4. Update or Create Daily Attendance Pair
	pair, err := uc.attendanceRepo.FindOpenPair(ctx, event.UserID, workDate)
	if err != nil || pair == nil {
		// Create new pair
		newPair := &domain.AttendancePair{
			UserID:    event.UserID,
			WorkDate:  workDate,
			CreatedAt: time.Now().UTC(),
			UpdatedAt: time.Now().UTC(),
		}

		if event.Direction == "OUT" {
			newPair.CheckOutEventID = &event.EventID
			newPair.CheckOutTime = &event.CreatedAt
			newPair.Status = "closed"
		} else {
			newPair.CheckInEventID = &event.EventID
			newPair.CheckInTime = &event.CreatedAt
			newPair.Status = "open"
		}
		return uc.attendanceRepo.SavePair(ctx, newPair)
	}

	// Existing pair
	now := time.Now().UTC()
	pair.UpdatedAt = now

	if event.Direction == "OUT" || (event.Direction == "AUTO" && pair.CheckInEventID != nil) {
		pair.CheckOutEventID = &event.EventID
		pair.CheckOutTime = &event.CreatedAt
		pair.Status = "closed"

		if pair.CheckInTime != nil {
			duration := event.CreatedAt.Sub(*pair.CheckInTime)
			hours := duration.Hours()
			if hours < 0 {
				hours = 0
			}
			pair.HoursWorked = &hours
		}
	} else if pair.CheckInEventID == nil {
		pair.CheckInEventID = &event.EventID
		pair.CheckInTime = &event.CreatedAt
	}

	return uc.attendanceRepo.UpdatePair(ctx, pair)
}
