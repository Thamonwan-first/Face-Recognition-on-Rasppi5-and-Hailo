package usecase

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"attendance-backend/internal/entity"
	"attendance-backend/internal/ports"
	"github.com/google/uuid"
)

type MeetingUseCase struct {
	meetingRepo ports.MeetingRepository
	deviceRepo  ports.DeviceRepository
	userRepo    ports.UserRepository
	auditRepo   ports.AuditRepository
}

func NewMeetingUseCase(
	meetingRepo ports.MeetingRepository,
	deviceRepo ports.DeviceRepository,
	userRepo ports.UserRepository,
	auditRepo ports.AuditRepository,
) *MeetingUseCase {
	return &MeetingUseCase{
		meetingRepo: meetingRepo,
		deviceRepo:  deviceRepo,
		userRepo:    userRepo,
		auditRepo:   auditRepo,
	}
}

type CreateMeetingDTO struct {
	Title                string  `json:"title"`
	Description          string  `json:"description"`
	Location             string  `json:"location"`
	DeviceID             *string `json:"device_id"`
	MeetingDate          string  `json:"meeting_date"` // YYYY-MM-DD
	StartTime            string  `json:"start_time"`   // HH:MM
	EndTime              string  `json:"end_time"`     // HH:MM
	LateThresholdMinutes int     `json:"late_threshold_minutes"`
}

type UpdateMeetingDTO struct {
	Title                string  `json:"title"`
	Description          string  `json:"description"`
	Location             string  `json:"location"`
	DeviceID             *string `json:"device_id"`
	MeetingDate          string  `json:"meeting_date"`
	StartTime            string  `json:"start_time"`
	EndTime              string  `json:"end_time"`
	LateThresholdMinutes int     `json:"late_threshold_minutes"`
	Status               string  `json:"status"` // "scheduled", "ongoing", "completed", "cancelled"
}

func (uc *MeetingUseCase) Create(ctx context.Context, adminID string, dto *CreateMeetingDTO) (*entity.Meeting, error) {
	if strings.TrimSpace(dto.Title) == "" {
		return nil, errors.New("หัวข้อการประชุมต้องไม่เป็นค่าว่าง")
	}
	if strings.TrimSpace(dto.MeetingDate) == "" {
		return nil, errors.New("กรุณาระบุวันที่ประชุม")
	}
	if strings.TrimSpace(dto.StartTime) == "" || strings.TrimSpace(dto.EndTime) == "" {
		return nil, errors.New("กรุณาระบุเวลาเริ่มและเวลาสิ้นสุดการประชุม")
	}
	if dto.LateThresholdMinutes < 0 {
		dto.LateThresholdMinutes = 15
	}

	now := time.Now().UTC()
	meeting := &entity.Meeting{
		ID:                   uuid.NewString(),
		Title:                strings.TrimSpace(dto.Title),
		Description:          strings.TrimSpace(dto.Description),
		Location:             strings.TrimSpace(dto.Location),
		DeviceID:             dto.DeviceID,
		MeetingDate:          strings.TrimSpace(dto.MeetingDate),
		StartTime:            strings.TrimSpace(dto.StartTime),
		EndTime:              strings.TrimSpace(dto.EndTime),
		LateThresholdMinutes: dto.LateThresholdMinutes,
		Status:               "scheduled",
		CreatedBy:            adminID,
		CreatedAt:            now,
		UpdatedAt:            now,
	}

	if err := uc.meetingRepo.Create(ctx, meeting); err != nil {
		return nil, err
	}

	if uc.auditRepo != nil {
		adminRef := adminID
		_ = uc.auditRepo.Log(ctx, &entity.AuditLog{
			AdminID:    &adminRef,
			Action:     "create_meeting",
			TargetType: "meeting",
			TargetID:   meeting.ID,
			Detail:     fmt.Sprintf("สร้างการประชุม: %s (%s %s-%s)", meeting.Title, meeting.MeetingDate, meeting.StartTime, meeting.EndTime),
			CreatedAt:  now,
		})
	}

	return meeting, nil
}

func (uc *MeetingUseCase) Update(ctx context.Context, adminID string, id string, dto *UpdateMeetingDTO) (*entity.Meeting, error) {
	meeting, err := uc.meetingRepo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	if strings.TrimSpace(dto.Title) != "" {
		meeting.Title = strings.TrimSpace(dto.Title)
	}
	meeting.Description = strings.TrimSpace(dto.Description)
	meeting.Location = strings.TrimSpace(dto.Location)
	meeting.DeviceID = dto.DeviceID

	if strings.TrimSpace(dto.MeetingDate) != "" {
		meeting.MeetingDate = strings.TrimSpace(dto.MeetingDate)
	}
	if strings.TrimSpace(dto.StartTime) != "" {
		meeting.StartTime = strings.TrimSpace(dto.StartTime)
	}
	if strings.TrimSpace(dto.EndTime) != "" {
		meeting.EndTime = strings.TrimSpace(dto.EndTime)
	}
	if dto.LateThresholdMinutes >= 0 {
		meeting.LateThresholdMinutes = dto.LateThresholdMinutes
	}
	if dto.Status != "" {
		meeting.Status = dto.Status
	}
	meeting.UpdatedAt = time.Now().UTC()

	if err := uc.meetingRepo.Update(ctx, meeting); err != nil {
		return nil, err
	}

	if uc.auditRepo != nil {
		adminRef := adminID
		_ = uc.auditRepo.Log(ctx, &entity.AuditLog{
			AdminID:    &adminRef,
			Action:     "update_meeting",
			TargetType: "meeting",
			TargetID:   meeting.ID,
			Detail:     fmt.Sprintf("แก้ไขการประชุม: %s", meeting.Title),
			CreatedAt:  meeting.UpdatedAt,
		})
	}

	return meeting, nil
}

// ComputeEffectiveStatus calculates the current operational status of a meeting based on Asia/Bangkok time.
func ComputeEffectiveStatus(m *entity.Meeting, now time.Time) string {
	if m == nil {
		return ""
	}
	if m.Status == "cancelled" {
		return "cancelled"
	}
	if m.MeetingDate == "" || m.StartTime == "" || m.EndTime == "" {
		return m.Status
	}

	loc := time.FixedZone("Asia/Bangkok", 7*3600)
	localNow := now.In(loc)
	todayStr := localNow.Format("2006-01-02")
	timeStr := localNow.Format("15:04")

	if m.MeetingDate < todayStr {
		return "completed"
	}
	if m.MeetingDate > todayStr {
		return "scheduled"
	}

	// Same day: compare start and end times (HH:MM)
	if timeStr < m.StartTime {
		return "scheduled"
	}
	if timeStr > m.EndTime {
		return "completed"
	}
	return "ongoing"
}

func (uc *MeetingUseCase) GetByID(ctx context.Context, id string) (*entity.Meeting, error) {
	m, err := uc.meetingRepo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	loc := time.FixedZone("Asia/Bangkok", 7*3600)
	m.Status = ComputeEffectiveStatus(m, time.Now().In(loc))
	return m, nil
}

func (uc *MeetingUseCase) List(ctx context.Context, date, status, deviceID string) ([]*entity.Meeting, error) {
	meetings, err := uc.meetingRepo.List(ctx, date, "", deviceID)
	if err != nil {
		return nil, err
	}
	loc := time.FixedZone("Asia/Bangkok", 7*3600)
	now := time.Now().In(loc)

	res := make([]*entity.Meeting, 0, len(meetings))
	for _, m := range meetings {
		m.Status = ComputeEffectiveStatus(m, now)
		if status != "" && status != "ALL" && m.Status != status {
			continue
		}
		res = append(res, m)
	}
	return res, nil
}

func (uc *MeetingUseCase) AutoSyncStatuses(ctx context.Context) (int, error) {
	meetings, err := uc.meetingRepo.List(ctx, "", "", "")
	if err != nil {
		return 0, err
	}

	loc := time.FixedZone("Asia/Bangkok", 7*3600)
	now := time.Now().In(loc)
	changedCount := 0

	for _, m := range meetings {
		if m.Status == "cancelled" {
			continue
		}
		newStatus := ComputeEffectiveStatus(m, now)
		if newStatus != "" && newStatus != m.Status {
			m.Status = newStatus
			m.UpdatedAt = now.UTC()
			if err := uc.meetingRepo.Update(ctx, m); err == nil {
				changedCount++
			}
		}
	}

	return changedCount, nil
}

func (uc *MeetingUseCase) Delete(ctx context.Context, adminID string, id string) error {
	m, _ := uc.meetingRepo.GetByID(ctx, id)
	title := ""
	if m != nil {
		title = m.Title
	}

	if err := uc.meetingRepo.Delete(ctx, id); err != nil {
		return err
	}

	if uc.auditRepo != nil {
		adminRef := adminID
		_ = uc.auditRepo.Log(ctx, &entity.AuditLog{
			AdminID:    &adminRef,
			Action:     "delete_meeting",
			TargetType: "meeting",
			TargetID:   id,
			Detail:     fmt.Sprintf("ลบการประชุม: %s (ID: %s)", title, id),
			CreatedAt:  time.Now().UTC(),
		})
	}
	return nil
}

func (uc *MeetingUseCase) ListAttendees(ctx context.Context, meetingID string) ([]*entity.MeetingAttendee, error) {
	return uc.meetingRepo.ListAttendees(ctx, meetingID)
}

// RecordMeetingAttendance handles face recognition check-in for meetings.
func (uc *MeetingUseCase) RecordMeetingAttendance(ctx context.Context, event *entity.AttendanceEvent) (*entity.MeetingAttendee, error) {
	loc := time.FixedZone("Asia/Bangkok", 7*3600)
	scanTime := event.CreatedAt.In(loc)
	scanDate := scanTime.Format("2006-01-02")
	scanTimeStr := scanTime.Format("15:04")

	// 1. Find active meeting on this device around current time
	meeting, err := uc.meetingRepo.FindActiveMeetingByDevice(ctx, event.DeviceID, scanDate, scanTimeStr)
	if err != nil || meeting == nil {
		// Fallback: search for any ongoing meeting today on this device
		meetings, listErr := uc.meetingRepo.List(ctx, scanDate, "", event.DeviceID)
		if listErr == nil && len(meetings) > 0 {
			for _, m := range meetings {
				if m.Status != "cancelled" && m.Status != "completed" {
					meeting = m
					break
				}
			}
		}
	}

	if meeting == nil {
		return nil, fmt.Errorf("ไม่พบวาระการประชุมที่เปิดรับลงทะเบียนในเวลานี้สำหรับเครื่อง %s", event.DeviceID)
	}

	// 2. Calculate on_time or late
	status := "on_time"
	minutesLate := 0

	// Parse start time
	startT, err := time.ParseInLocation("2006-01-02 15:04", fmt.Sprintf("%s %s", meeting.MeetingDate, meeting.StartTime), loc)
	if err == nil {
		lateLimit := startT.Add(time.Duration(meeting.LateThresholdMinutes) * time.Minute)
		if scanTime.After(lateLimit) {
			status = "late"
			minutesLate = int(scanTime.Sub(startT).Minutes())
		}
	}

	attendee := &entity.MeetingAttendee{
		MeetingID:   meeting.ID,
		UserID:      event.UserID,
		EventID:     event.EventID,
		ScannedAt:   event.CreatedAt,
		SnapshotURL: event.SnapshotURL,
		Status:      status,
		MinutesLate: minutesLate,
		Notes:       event.Notes,
		CreatedAt:   time.Now().UTC(),
	}

	if err := uc.meetingRepo.RecordAttendee(ctx, attendee); err != nil {
		return nil, err
	}

	return attendee, nil
}
