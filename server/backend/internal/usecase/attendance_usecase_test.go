package usecase_test

import (
	"context"
	"testing"
	"time"

	"attendance-backend/internal/core/domain"
	"attendance-backend/internal/usecase"
)

type mockAttendanceRepo struct {
	events []*domain.AttendanceEvent
	pairs  []*domain.AttendancePair
}

func (m *mockAttendanceRepo) SaveEvent(ctx context.Context, event *domain.AttendanceEvent) error {
	m.events = append(m.events, event)
	return nil
}

func (m *mockAttendanceRepo) GetEventByID(ctx context.Context, eventID string) (*domain.AttendanceEvent, error) {
	for _, e := range m.events {
		if e.EventID == eventID {
			return e, nil
		}
	}
	return nil, domain.ErrNotFound
}

func (m *mockAttendanceRepo) ListEvents(ctx context.Context, limit int) ([]*domain.AttendanceEvent, error) {
	return m.events, nil
}

func (m *mockAttendanceRepo) ListPairs(ctx context.Context, startDate, endDate, userID, department string, limit int) ([]*domain.AttendancePair, error) {
	return m.pairs, nil
}

func (m *mockAttendanceRepo) FindOpenPair(ctx context.Context, userID, workDate string) (*domain.AttendancePair, error) {
	for _, p := range m.pairs {
		if p.UserID == userID && p.WorkDate == workDate {
			return p, nil
		}
	}
	return nil, domain.ErrNotFound
}

func (m *mockAttendanceRepo) SavePair(ctx context.Context, pair *domain.AttendancePair) error {
	pair.ID = len(m.pairs) + 1
	m.pairs = append(m.pairs, pair)
	return nil
}

func (m *mockAttendanceRepo) UpdatePair(ctx context.Context, pair *domain.AttendancePair) error {
	return nil
}

func TestAttendanceUseCase_ProcessEvent(t *testing.T) {
	repo := &mockAttendanceRepo{}
	uc := usecase.NewAttendanceUseCase(repo)

	now := time.Now().UTC()
	eventIn := &domain.AttendanceEvent{
		EventID:   "evt-1",
		UserID:    "user-100",
		Direction: "IN",
		CreatedAt: now,
	}

	err := uc.ProcessEvent(context.Background(), eventIn)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(repo.pairs) != 1 {
		t.Fatalf("expected 1 pair created, got %d", len(repo.pairs))
	}
	if *repo.pairs[0].CheckInEventID != "evt-1" {
		t.Fatalf("expected check-in event ID evt-1, got %s", *repo.pairs[0].CheckInEventID)
	}

	// Now check out
	eventOut := &domain.AttendanceEvent{
		EventID:   "evt-2",
		UserID:    "user-100",
		Direction: "OUT",
		CreatedAt: now.Add(8 * time.Hour),
	}

	err = uc.ProcessEvent(context.Background(), eventOut)
	if err != nil {
		t.Fatalf("expected no error on checkout, got %v", err)
	}

	if repo.pairs[0].CheckOutEventID == nil || *repo.pairs[0].CheckOutEventID != "evt-2" {
		t.Fatalf("expected check-out event ID evt-2")
	}
	if repo.pairs[0].HoursWorked == nil || *repo.pairs[0].HoursWorked < 7.9 {
		t.Fatalf("expected ~8 hours worked")
	}
}
