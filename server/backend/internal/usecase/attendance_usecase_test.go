package usecase_test

import (
	"context"
	"testing"
	"time"

	"attendance-backend/internal/entity"
	"attendance-backend/internal/usecase"
)

type mockAttendanceRepo struct {
	events []*entity.AttendanceEvent
	pairs  []*entity.AttendancePair
}

func (m *mockAttendanceRepo) SaveEvent(ctx context.Context, event *entity.AttendanceEvent) error {
	m.events = append(m.events, event)
	return nil
}

func (m *mockAttendanceRepo) GetEventByID(ctx context.Context, eventID string) (*entity.AttendanceEvent, error) {
	for _, e := range m.events {
		if e.EventID == eventID {
			return e, nil
		}
	}
	return nil, entity.ErrNotFound
}

func (m *mockAttendanceRepo) ListEvents(ctx context.Context, limit int) ([]*entity.AttendanceEvent, error) {
	return m.events, nil
}

func (m *mockAttendanceRepo) ListPairs(ctx context.Context, startDate, endDate, userID, department, deviceID string, limit int) ([]*entity.AttendancePair, error) {
	return m.pairs, nil
}

func (m *mockAttendanceRepo) GetPairByID(ctx context.Context, id int) (*entity.AttendancePair, error) {
	for _, p := range m.pairs {
		if p.ID == id {
			return p, nil
		}
	}
	return nil, entity.ErrNotFound
}

func (m *mockAttendanceRepo) FindOpenPair(ctx context.Context, userID, workDate string) (*entity.AttendancePair, error) {
	for _, p := range m.pairs {
		if p.UserID == userID && p.WorkDate == workDate && p.Status == "open" {
			return p, nil
		}
	}
	return nil, entity.ErrNotFound
}

func (m *mockAttendanceRepo) FindLastPairByUserAndDate(ctx context.Context, userID, workDate string) (*entity.AttendancePair, error) {
	var best *entity.AttendancePair
	for _, p := range m.pairs {
		if p.UserID == userID && p.WorkDate == workDate {
			if best == nil || (best.CheckInEventID == nil && p.CheckInEventID != nil) || p.ID > best.ID {
				best = p
			}
		}
	}
	if best != nil {
		return best, nil
	}
	return nil, entity.ErrNotFound
}

func (m *mockAttendanceRepo) SavePair(ctx context.Context, pair *entity.AttendancePair) error {
	pair.ID = len(m.pairs) + 1
	m.pairs = append(m.pairs, pair)
	return nil
}

func (m *mockAttendanceRepo) UpdatePair(ctx context.Context, pair *entity.AttendancePair) error {
	return nil
}

type mockUserRepo struct {
	users []*entity.User
}

func (m *mockUserRepo) GetByID(ctx context.Context, id string) (*entity.User, error) { return nil, nil }
func (m *mockUserRepo) GetByEmployeeID(ctx context.Context, employeeID string) (*entity.User, error) {
	return nil, nil
}
func (m *mockUserRepo) List(ctx context.Context, unenrolledOnly bool) ([]*entity.User, error) {
	return m.users, nil
}
func (m *mockUserRepo) Create(ctx context.Context, user *entity.User) error { return nil }
func (m *mockUserRepo) Update(ctx context.Context, user *entity.User) error { return nil }
func (m *mockUserRepo) Delete(ctx context.Context, id string) error         { return nil }
func (m *mockUserRepo) GetLatestVersion(ctx context.Context) (int, error)   { return 1, nil }
func (m *mockUserRepo) ListUpdatedSince(ctx context.Context, sinceVersion int) ([]*entity.User, error) {
	return nil, nil
}
func (m *mockUserRepo) SaveEmbedding(ctx context.Context, embedding *entity.UserEmbedding) error {
	return nil
}
func (m *mockUserRepo) GetEmbedding(ctx context.Context, userID string) (*entity.UserEmbedding, error) {
	return nil, nil
}
func (m *mockUserRepo) DeleteEmbedding(ctx context.Context, userID string) error {
	return nil
}

func TestAttendanceUseCase_ProcessEvent(t *testing.T) {
	repo := &mockAttendanceRepo{}
	uc := usecase.NewAttendanceUseCase(repo)

	now := time.Now().UTC()
	eventIn := &entity.AttendanceEvent{
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
	eventOut := &entity.AttendanceEvent{
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

	// Test second session (e.g. after lunch return) on the same day
	eventIn2 := &entity.AttendanceEvent{
		EventID:   "evt-3",
		UserID:    "user-100",
		Direction: "IN",
		CreatedAt: now.Add(9 * time.Hour),
	}
	err = uc.ProcessEvent(context.Background(), eventIn2)
	if err != nil {
		t.Fatalf("expected no error on second check-in, got %v", err)
	}

	if len(repo.pairs) != 2 {
		t.Fatalf("expected 2 separate pairs for multiple sessions, got %d", len(repo.pairs))
	}
	if repo.pairs[1].Status != "open" {
		t.Fatalf("expected second pair to be open, got %s", repo.pairs[1].Status)
	}
}

func TestAttendanceUseCase_ProcessEvent_MultipleOuts(t *testing.T) {
	repo := &mockAttendanceRepo{}
	uc := usecase.NewAttendanceUseCase(repo)

	now := time.Now().UTC()
	// 1. First scan: IN
	eventIn := &entity.AttendanceEvent{
		EventID:   "evt-in-1",
		UserID:    "user-200",
		Direction: "IN",
		CreatedAt: now,
	}
	if err := uc.ProcessEvent(context.Background(), eventIn); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(repo.pairs) != 1 {
		t.Fatalf("expected 1 pair, got %d", len(repo.pairs))
	}

	// 2. Second scan: OUT (e.g. 1 hour later)
	eventOut1 := &entity.AttendanceEvent{
		EventID:   "evt-out-1",
		UserID:    "user-200",
		Direction: "OUT",
		CreatedAt: now.Add(1 * time.Hour),
	}
	if err := uc.ProcessEvent(context.Background(), eventOut1); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(repo.pairs) != 1 {
		t.Fatalf("expected still 1 pair after first checkout, got %d", len(repo.pairs))
	}
	if *repo.pairs[0].CheckOutEventID != "evt-out-1" {
		t.Fatalf("expected check-out evt-out-1, got %s", *repo.pairs[0].CheckOutEventID)
	}

	// 3. Third scan: OUT again (e.g. at end of day, 8 hours later)
	eventOut2 := &entity.AttendanceEvent{
		EventID:   "evt-out-2",
		UserID:    "user-200",
		Direction: "OUT",
		CreatedAt: now.Add(8 * time.Hour),
	}
	if err := uc.ProcessEvent(context.Background(), eventOut2); err != nil {
		t.Fatalf("expected no error on duplicate checkout, got %v", err)
	}

	// Crucial check: Must NOT create a second pair!
	if len(repo.pairs) != 1 {
		t.Fatalf("expected still 1 pair (no extra rows), but got %d", len(repo.pairs))
	}
	if *repo.pairs[0].CheckOutEventID != "evt-out-2" {
		t.Fatalf("expected updated check-out evt-out-2, got %s", *repo.pairs[0].CheckOutEventID)
	}
	if repo.pairs[0].HoursWorked == nil || *repo.pairs[0].HoursWorked < 7.9 {
		t.Fatalf("expected updated hours worked ~8, got %v", repo.pairs[0].HoursWorked)
	}
}

func TestAttendanceUseCase_CalculateStatus(t *testing.T) {
	loc := time.FixedZone("ICT", 7*3600)

	// 1. On Time: check in 08:15, check out 17:15 (9 hrs worked)
	tIn := time.Date(2026, 11, 4, 8, 15, 0, 0, loc).UTC()
	tOut := time.Date(2026, 11, 4, 17, 15, 0, 0, loc).UTC()
	hrs := 9.0
	status := usecase.CalculateAttendanceStatus(&tIn, &tOut, &hrs, false)
	if status != "on_time" {
		t.Fatalf("expected on_time, got %s", status)
	}

	// 2. Late: check in 08:45, check out 17:45 (9 hrs worked)
	tInLate := time.Date(2026, 11, 4, 8, 45, 0, 0, loc).UTC()
	statusLate := usecase.CalculateAttendanceStatus(&tInLate, &tOut, &hrs, false)
	if statusLate != "late" {
		t.Fatalf("expected late, got %s", statusLate)
	}

	// 3. Incomplete: check in 08:15, check out 12:15 (4 hrs worked)
	hrsShort := 4.0
	statusShort := usecase.CalculateAttendanceStatus(&tIn, &tOut, &hrsShort, false)
	if statusShort != "incomplete" {
		t.Fatalf("expected incomplete, got %s", statusShort)
	}

	// 4. Open: check in only
	statusOpen := usecase.CalculateAttendanceStatus(&tIn, nil, nil, false)
	if statusOpen != "open" {
		t.Fatalf("expected open, got %s", statusOpen)
	}

	// 5. Manual: isManual = true
	statusManual := usecase.CalculateAttendanceStatus(&tIn, &tOut, &hrs, true)
	if statusManual != "adjusted" {
		t.Fatalf("expected adjusted, got %s", statusManual)
	}
}

func TestAttendanceUseCase_ManualAdjustment(t *testing.T) {
	repo := &mockAttendanceRepo{}
	uc := usecase.NewAttendanceUseCase(repo)

	// 1. Missing reason must fail
	_, err := uc.ManualAddAttendance(context.Background(), "admin-1", &usecase.ManualAttendanceDTO{
		UserID:   "user-1",
		WorkDate: "2026-11-04",
		Reason:   "",
	})
	if err == nil {
		t.Fatalf("expected error when reason is empty")
	}

	// 2. Add with reason
	pair, err := uc.ManualAddAttendance(context.Background(), "admin-1", &usecase.ManualAttendanceDTO{
		UserID:       "user-1",
		WorkDate:     "2026-11-04",
		CheckInTime:  "08:30",
		CheckOutTime: "17:30",
		Reason:       "พนักงานลืมสแกนช่วงเช้าเนื่องจากพบลูกค้า",
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if !pair.IsManual {
		t.Fatalf("expected is_manual to be true")
	}
	if pair.Status != "adjusted" {
		t.Fatalf("expected status adjusted, got %s", pair.Status)
	}
	if pair.AdjustmentReason != "พนักงานลืมสแกนช่วงเช้าเนื่องจากพบลูกค้า" {
		t.Fatalf("unexpected reason: %s", pair.AdjustmentReason)
	}

	// 3. Edit with reason
	updated, err := uc.ManualEditAttendance(context.Background(), "admin-1", pair.ID, &usecase.UpdateAttendancePairDTO{
		CheckInTime:  "08:00",
		CheckOutTime: "17:00",
		Reason:       "แก้ไขเวลาจริงหลังจากตรวจสอบกับหัวหน้างาน",
	})
	if err != nil {
		t.Fatalf("expected no error on edit, got %v", err)
	}
	if updated.AdjustmentReason != "แก้ไขเวลาจริงหลังจากตรวจสอบกับหัวหน้างาน" {
		t.Fatalf("unexpected reason: %s", updated.AdjustmentReason)
	}
}

func TestAttendanceUseCase_AbsentDetection(t *testing.T) {
	repo := &mockAttendanceRepo{
		pairs: []*entity.AttendancePair{
			{
				ID:       1,
				UserID:   "user-attended",
				WorkDate: "2026-11-04",
				Status:   "on_time",
			},
		},
	}
	userRepo := &mockUserRepo{
		users: []*entity.User{
			{ID: "user-attended", Name: "Somchai", Status: "active"},
			{ID: "user-absent", Name: "Somsak", Status: "active"},
			{ID: "user-inactive", Name: "InactivePerson", Status: "inactive"},
		},
	}
	uc := usecase.NewAttendanceUseCase(repo, userRepo)

	// ListPairs with startDate = 2026-11-04, includeAbsent = true
	pairs, err := uc.ListPairs(context.Background(), "2026-11-04", "2026-11-04", "", "", "", true, 100)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	// Should have 2 pairs: 1 attended + 1 absent (inactive person ignored)
	if len(pairs) != 2 {
		t.Fatalf("expected 2 pairs (attended + absent), got %d", len(pairs))
	}

	foundAbsent := false
	for _, p := range pairs {
		if p.UserID == "user-absent" {
			foundAbsent = true
			if p.Status != "absent" {
				t.Fatalf("expected status absent, got %s", p.Status)
			}
		}
	}
	if !foundAbsent {
		t.Fatalf("expected user-absent to be included")
	}
}
