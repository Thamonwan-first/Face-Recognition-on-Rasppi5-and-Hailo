package usecase_test

import (
	"context"
	"testing"
	"time"

	"attendance-backend/internal/entity"
	"attendance-backend/internal/usecase"
)

type mockMeetingRepo struct {
	meetings  []*entity.Meeting
	attendees []*entity.MeetingAttendee
}

func (m *mockMeetingRepo) Create(ctx context.Context, meeting *entity.Meeting) error {
	m.meetings = append(m.meetings, meeting)
	return nil
}

func (m *mockMeetingRepo) Update(ctx context.Context, meeting *entity.Meeting) error {
	for i, mt := range m.meetings {
		if mt.ID == meeting.ID {
			m.meetings[i] = meeting
			return nil
		}
	}
	return entity.ErrNotFound
}

func (m *mockMeetingRepo) GetByID(ctx context.Context, id string) (*entity.Meeting, error) {
	for _, mt := range m.meetings {
		if mt.ID == id {
			return mt, nil
		}
	}
	return nil, entity.ErrNotFound
}

func (m *mockMeetingRepo) List(ctx context.Context, date string, status string, deviceID string) ([]*entity.Meeting, error) {
	var res []*entity.Meeting
	for _, mt := range m.meetings {
		if date != "" && mt.MeetingDate != date {
			continue
		}
		if status != "" && mt.Status != status {
			continue
		}
		if deviceID != "" && (mt.DeviceID == nil || *mt.DeviceID != deviceID) {
			continue
		}
		res = append(res, mt)
	}
	return res, nil
}

func (m *mockMeetingRepo) Delete(ctx context.Context, id string) error {
	for i, mt := range m.meetings {
		if mt.ID == id {
			m.meetings = append(m.meetings[:i], m.meetings[i+1:]...)
			return nil
		}
	}
	return nil
}

func (m *mockMeetingRepo) FindActiveMeetingByDevice(ctx context.Context, deviceID string, date string, timeStr string) (*entity.Meeting, error) {
	for _, mt := range m.meetings {
		if mt.DeviceID != nil && *mt.DeviceID == deviceID && mt.MeetingDate == date && mt.Status != "cancelled" {
			return mt, nil
		}
	}
	return nil, entity.ErrNotFound
}

func (m *mockMeetingRepo) RecordAttendee(ctx context.Context, attendee *entity.MeetingAttendee) error {
	for i, att := range m.attendees {
		if att.MeetingID == attendee.MeetingID && att.UserID == attendee.UserID {
			m.attendees[i] = attendee
			return nil
		}
	}
	attendee.ID = len(m.attendees) + 1
	m.attendees = append(m.attendees, attendee)
	return nil
}

func (m *mockMeetingRepo) ListAttendees(ctx context.Context, meetingID string) ([]*entity.MeetingAttendee, error) {
	var res []*entity.MeetingAttendee
	for _, att := range m.attendees {
		if att.MeetingID == meetingID {
			res = append(res, att)
		}
	}
	return res, nil
}

func (m *mockMeetingRepo) GetAttendee(ctx context.Context, meetingID string, userID string) (*entity.MeetingAttendee, error) {
	for _, att := range m.attendees {
		if att.MeetingID == meetingID && att.UserID == userID {
			return att, nil
		}
	}
	return nil, entity.ErrNotFound
}

type mockDeviceRepo struct {
	devices []*entity.Device
}

func (m *mockDeviceRepo) GetByID(ctx context.Context, id string) (*entity.Device, error) {
	for _, d := range m.devices {
		if d.ID == id {
			return d, nil
		}
	}
	return nil, entity.ErrNotFound
}
func (m *mockDeviceRepo) GetByApiKeyHash(ctx context.Context, hash string) (*entity.Device, error) {
	return nil, nil
}
func (m *mockDeviceRepo) List(ctx context.Context) ([]*entity.Device, error) {
	return m.devices, nil
}
func (m *mockDeviceRepo) Create(ctx context.Context, device *entity.Device) error { return nil }
func (m *mockDeviceRepo) Update(ctx context.Context, device *entity.Device) error {
	for i, d := range m.devices {
		if d.ID == device.ID {
			m.devices[i] = device
			return nil
		}
	}
	return nil
}
func (m *mockDeviceRepo) UpdateStatus(ctx context.Context, id string, status string, lastSeen time.Time) error {
	return nil
}
func (m *mockDeviceRepo) UpdateIPAddress(ctx context.Context, id string, ip string) error { return nil }
func (m *mockDeviceRepo) UpdateStatusAndIP(ctx context.Context, id string, status string, ip string, lastSeen time.Time) error {
	return nil
}
func (m *mockDeviceRepo) UpdateState(ctx context.Context, state *entity.DeviceState) error {
	return nil
}
func (m *mockDeviceRepo) UpdateMode(ctx context.Context, id string, mode string) error {
	for _, d := range m.devices {
		if d.ID == id {
			d.Mode = mode
			return nil
		}
	}
	return entity.ErrNotFound
}
func (m *mockDeviceRepo) Delete(ctx context.Context, id string) error { return nil }

func TestMeetingUseCase_CreateAndRecordAttendee(t *testing.T) {
	meetRepo := &mockMeetingRepo{}
	devRepo := &mockDeviceRepo{
		devices: []*entity.Device{
			{ID: "kiosk-meet-1", Name: "Meeting Room A", Mode: "meeting"},
		},
	}
	userRepo := &mockUserRepo{
		users: []*entity.User{
			{ID: "usr-1", Name: "Somchai", EmployeeID: "EMP001", Department: "Sales"},
		},
	}

	uc := usecase.NewMeetingUseCase(meetRepo, devRepo, userRepo, nil)

	devID := "kiosk-meet-1"
	m, err := uc.Create(context.Background(), "admin-1", &usecase.CreateMeetingDTO{
		Title:                "Weekly Sales Meeting",
		Description:          "Q4 review",
		Location:             "Room A",
		DeviceID:             &devID,
		MeetingDate:          "2026-11-20",
		StartTime:            "13:30",
		EndTime:              "15:00",
		LateThresholdMinutes: 15,
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if m.ID == "" || m.Title != "Weekly Sales Meeting" {
		t.Fatalf("unexpected meeting: %+v", m)
	}

	// 1. On-time scan (13:25)
	loc := time.FixedZone("Asia/Bangkok", 7*3600)
	onTime := time.Date(2026, 11, 20, 13, 25, 0, 0, loc).UTC()
	att, err := uc.RecordMeetingAttendance(context.Background(), &entity.AttendanceEvent{
		EventID:     "evt-meet-1",
		UserID:      "usr-1",
		DeviceID:    "kiosk-meet-1",
		SnapshotURL: "/uploads/snap1.jpg",
		CreatedAt:   onTime,
	})
	if err != nil {
		t.Fatalf("expected no error recording attendance, got %v", err)
	}
	if att.Status != "on_time" {
		t.Fatalf("expected status on_time, got %s", att.Status)
	}
	if att.MinutesLate != 0 {
		t.Fatalf("expected 0 minutes late, got %d", att.MinutesLate)
	}

	// 2. Late scan (13:50, after 15m threshold)
	lateTime := time.Date(2026, 11, 20, 13, 50, 0, 0, loc).UTC()
	attLate, err := uc.RecordMeetingAttendance(context.Background(), &entity.AttendanceEvent{
		EventID:     "evt-meet-2",
		UserID:      "usr-2",
		DeviceID:    "kiosk-meet-1",
		SnapshotURL: "/uploads/snap2.jpg",
		CreatedAt:   lateTime,
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if attLate.Status != "late" {
		t.Fatalf("expected status late, got %s", attLate.Status)
	}
	if attLate.MinutesLate < 19 {
		t.Fatalf("expected ~20 minutes late, got %d", attLate.MinutesLate)
	}

	// Check attendee list
	list, err := uc.ListAttendees(context.Background(), m.ID)
	if err != nil {
		t.Fatalf("expected no error listing attendees, got %v", err)
	}
	if len(list) != 2 {
		t.Fatalf("expected 2 attendees, got %d", len(list))
	}
}

func TestAttendanceUseCase_MeetingModeIsolation(t *testing.T) {
	// Verify that when a device is in "meeting" mode, scanning does NOT create or alter daily attendance pairs
	attRepo := &mockAttendanceRepo{}
	meetRepo := &mockMeetingRepo{}
	devRepo := &mockDeviceRepo{
		devices: []*entity.Device{
			{ID: "kiosk-meet-1", Name: "Meeting Room A", Mode: "meeting"},
			{ID: "kiosk-front-1", Name: "Front Door", Mode: "attendance"},
		},
	}
	userRepo := &mockUserRepo{
		users: []*entity.User{
			{ID: "usr-1", Name: "Somchai", Status: "active"},
		},
	}

	meetUC := usecase.NewMeetingUseCase(meetRepo, devRepo, userRepo, nil)
	devID := "kiosk-meet-1"
	_, _ = meetUC.Create(context.Background(), "admin-1", &usecase.CreateMeetingDTO{
		Title:       "Safety Briefing",
		DeviceID:    &devID,
		MeetingDate: "2026-11-20",
		StartTime:   "10:00",
		EndTime:     "11:00",
	})

	attUC := usecase.NewAttendanceUseCase(attRepo, devRepo, meetUC, userRepo)

	loc := time.FixedZone("Asia/Bangkok", 7*3600)
	scanTime := time.Date(2026, 11, 20, 10, 05, 0, 0, loc).UTC()

	// Employee scans at meeting kiosk
	err := attUC.ProcessEvent(context.Background(), &entity.AttendanceEvent{
		EventID:     "evt-scan-meeting",
		UserID:      "usr-1",
		DeviceID:    "kiosk-meet-1",
		SnapshotURL: "/uploads/meeting_snap.jpg",
		CreatedAt:   scanTime,
	})
	if err != nil {
		t.Fatalf("expected no error processing meeting event, got %v", err)
	}

	// 1. Meeting attendees MUST have 1 record
	if len(meetRepo.attendees) != 1 {
		t.Fatalf("expected meeting attendees to have 1 record, got %d", len(meetRepo.attendees))
	}

	// 2. Attendance pairs MUST BE EMPTY (complete isolation from daily work hours!)
	if len(attRepo.pairs) != 0 {
		t.Fatalf("expected attendance_pairs to remain EMPTY, but got %d pairs", len(attRepo.pairs))
	}
}

func TestMeetingUseCase_AutoSyncStatuses_AndTimeTransitions(t *testing.T) {
	meetRepo := &mockMeetingRepo{}
	devRepo := &mockDeviceRepo{}
	userRepo := &mockUserRepo{}

	meetUC := usecase.NewMeetingUseCase(meetRepo, devRepo, userRepo, nil)

	loc := time.FixedZone("Asia/Bangkok", 7*3600)
	now := time.Now().In(loc)
	todayStr := now.Format("2006-01-02")
	yesterdayStr := now.AddDate(0, 0, -1).Format("2006-01-02")
	tomorrowStr := now.AddDate(0, 0, 1).Format("2006-01-02")

	// Meeting 1: Yesterday -> should be completed
	mPast := &entity.Meeting{
		ID:          "m-past",
		Title:       "Past Meeting",
		MeetingDate: yesterdayStr,
		StartTime:   "09:00",
		EndTime:     "10:00",
		Status:      "scheduled",
	}

	// Meeting 2: Tomorrow -> should remain scheduled
	mFuture := &entity.Meeting{
		ID:          "m-future",
		Title:       "Future Meeting",
		MeetingDate: tomorrowStr,
		StartTime:   "09:00",
		EndTime:     "10:00",
		Status:      "scheduled",
	}

	// Meeting 3: Cancelled -> should remain cancelled
	mCancelled := &entity.Meeting{
		ID:          "m-cancelled",
		Title:       "Cancelled Meeting",
		MeetingDate: todayStr,
		StartTime:   "00:00",
		EndTime:     "23:59",
		Status:      "cancelled",
	}

	// Meeting 4: Today active window -> ongoing
	curHour := now.Hour()
	stHour := curHour - 1
	if stHour < 0 {
		stHour = 0
	}
	etHour := curHour + 1
	if etHour > 23 {
		etHour = 23
	}
	mActive := &entity.Meeting{
		ID:          "m-active",
		Title:       "Active Meeting",
		MeetingDate: todayStr,
		StartTime:   time.Date(now.Year(), now.Month(), now.Day(), stHour, 0, 0, 0, loc).Format("15:04"),
		EndTime:     time.Date(now.Year(), now.Month(), now.Day(), etHour, 59, 0, 0, loc).Format("15:04"),
		Status:      "scheduled",
	}

	meetRepo.meetings = []*entity.Meeting{mPast, mFuture, mCancelled, mActive}

	// Run AutoSyncStatuses
	changed, err := meetUC.AutoSyncStatuses(context.Background())
	if err != nil {
		t.Fatalf("unexpected error from AutoSyncStatuses: %v", err)
	}

	// mPast (scheduled -> completed) and mActive (scheduled -> ongoing) should have changed
	if changed != 2 {
		t.Errorf("expected 2 changed meetings, got %d", changed)
	}

	if mPast.Status != "completed" {
		t.Errorf("expected mPast to be 'completed', got '%s'", mPast.Status)
	}
	if mFuture.Status != "scheduled" {
		t.Errorf("expected mFuture to remain 'scheduled', got '%s'", mFuture.Status)
	}
	if mCancelled.Status != "cancelled" {
		t.Errorf("expected mCancelled to remain 'cancelled', got '%s'", mCancelled.Status)
	}
	if mActive.Status != "ongoing" {
		t.Errorf("expected mActive to be 'ongoing', got '%s'", mActive.Status)
	}

	// Test dynamic List enriched
	list, err := meetUC.List(context.Background(), "", "", "")
	if err != nil {
		t.Fatalf("unexpected error from List: %v", err)
	}
	for _, m := range list {
		if m.ID == "m-active" && m.Status != "ongoing" {
			t.Errorf("List should return dynamic ongoing for m-active, got %s", m.Status)
		}
	}
}
