package usecase

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"time"

	"attendance-backend/internal/entity"
	"attendance-backend/internal/ports"
)

type AttendanceUseCase struct {
	attendanceRepo ports.AttendanceRepository
	auditRepo      ports.AuditRepository
	userRepo       ports.UserRepository
	settingsRepo   ports.SettingsRepository
}

func NewAttendanceUseCase(
	attendanceRepo ports.AttendanceRepository,
	repos ...interface{},
) *AttendanceUseCase {
	uc := &AttendanceUseCase{attendanceRepo: attendanceRepo}
	for _, r := range repos {
		switch v := r.(type) {
		case ports.AuditRepository:
			uc.auditRepo = v
		case ports.UserRepository:
			uc.userRepo = v
		case ports.SettingsRepository:
			uc.settingsRepo = v
		}
	}
	return uc
}

type ManualAttendanceDTO struct {
	UserID       string `json:"user_id" binding:"required"`
	WorkDate     string `json:"work_date" binding:"required"`
	CheckInTime  string `json:"check_in_time"`
	CheckOutTime string `json:"check_out_time"`
	Reason       string `json:"reason" binding:"required"`
}

type UpdateAttendancePairDTO struct {
	CheckInTime  string `json:"check_in_time"`
	CheckOutTime string `json:"check_out_time"`
	Reason       string `json:"reason" binding:"required"`
}

// CalculateAttendanceStatusWithConfig determines work status with custom enterprise settings
func CalculateAttendanceStatusWithConfig(checkInTime, checkOutTime *time.Time, hoursWorked *float64, isManual bool, startHour, startMinute int, minHours float64) string {
	if isManual {
		return "adjusted"
	}
	if checkInTime == nil {
		return "incomplete"
	}
	if checkOutTime == nil {
		return "open"
	}

	loc := time.FixedZone("ICT", 7*3600)
	checkInLocal := checkInTime.In(loc)
	isLate := checkInLocal.Hour() > startHour || (checkInLocal.Hour() == startHour && checkInLocal.Minute() > startMinute)

	hours := 0.0
	if hoursWorked != nil {
		hours = *hoursWorked
	}

	if isLate {
		return "late"
	}
	if hours < minHours {
		return "incomplete"
	}
	return "on_time"
}

// CalculateAttendanceStatus determines work status based on default rules (08:30 ICT, 8.0 hrs)
func CalculateAttendanceStatus(checkInTime, checkOutTime *time.Time, hoursWorked *float64, isManual bool) string {
	return CalculateAttendanceStatusWithConfig(checkInTime, checkOutTime, hoursWorked, isManual, 8, 30, 8.0)
}

func parseDateTime(dateStr, timeStr string) (*time.Time, error) {
	if timeStr == "" {
		return nil, nil
	}
	// 1. Try ISO / RFC3339 format
	if t, err := time.Parse(time.RFC3339, timeStr); err == nil {
		tUTC := t.UTC()
		return &tUTC, nil
	}
	if t, err := time.Parse("2006-01-02T15:04:05", timeStr); err == nil {
		loc := time.FixedZone("ICT", 7*3600)
		tLocal := time.Date(t.Year(), t.Month(), t.Day(), t.Hour(), t.Minute(), t.Second(), 0, loc).UTC()
		return &tLocal, nil
	}
	// 2. Try "15:04" or "15:04:05" combined with dateStr (ICT UTC+7)
	loc := time.FixedZone("ICT", 7*3600)
	fullStr := dateStr + " " + timeStr
	if t, err := time.ParseInLocation("2006-01-02 15:04:05", fullStr, loc); err == nil {
		tUTC := t.UTC()
		return &tUTC, nil
	}
	if t, err := time.ParseInLocation("2006-01-02 15:04", fullStr, loc); err == nil {
		tUTC := t.UTC()
		return &tUTC, nil
	}
	return nil, fmt.Errorf("invalid time format: %s", timeStr)
}

func (uc *AttendanceUseCase) ListPairs(ctx context.Context, startDate, endDate, userID, department, deviceID string, includeAbsent bool, limit int) ([]*entity.AttendancePair, error) {
	if limit <= 0 {
		limit = 500
	}
	pairs, err := uc.attendanceRepo.ListPairs(ctx, startDate, endDate, userID, department, deviceID, limit)
	if err != nil {
		return nil, err
	}

	// Dynamic status calculation based on current system settings
	startH, startM := 8, 30
	minHrs := 8.0
	if uc.settingsRepo != nil {
		if s, err := uc.settingsRepo.GetSettings(ctx); err == nil && s != nil {
			if len(s.WorkStartTime) == 5 && s.WorkStartTime[2] == ':' {
				if h, err := strconv.Atoi(s.WorkStartTime[:2]); err == nil {
					if m, err := strconv.Atoi(s.WorkStartTime[3:]); err == nil {
						startH, startM = h, m
					}
				}
			}
			if s.MinWorkHours > 0 {
				minHrs = s.MinWorkHours
			}
		}
	}

	for _, p := range pairs {
		if !p.IsManual && p.Status != "adjusted" && p.Status != "open" {
			p.Status = CalculateAttendanceStatusWithConfig(p.CheckInTime, p.CheckOutTime, p.HoursWorked, false, startH, startM, minHrs)
		}
	}

	// Absent detection:
	// When includeAbsent is requested and date is specified (e.g. today or date filter)
	if includeAbsent && uc.userRepo != nil && startDate != "" && (startDate == endDate || endDate == "") {
		evalDate := startDate
		users, err := uc.userRepo.List(ctx, false)
		if err == nil {
			userWithPairs := make(map[string]bool)
			for _, p := range pairs {
				if p.WorkDate == evalDate {
					userWithPairs[p.UserID] = true
				}
			}

			for _, u := range users {
				if u.Status != "active" {
					continue
				}
				if userID != "" && u.ID != userID {
					continue
				}
				if department != "" && u.Department != department {
					continue
				}
				if u.StartDate != nil && evalDate < *u.StartDate {
					continue
				}
				if u.EndDate != nil && evalDate > *u.EndDate {
					continue
				}

				if !userWithPairs[u.ID] {
					pairs = append(pairs, &entity.AttendancePair{
						ID:         0,
						UserID:     u.ID,
						UserName:   u.Name,
						EmployeeID: u.EmployeeID,
						Department: u.Department,
						WorkDate:   evalDate,
						Status:     "absent",
						IsManual:   false,
					})
				}
			}
		}
	}

	return pairs, nil
}

func (uc *AttendanceUseCase) ListEvents(ctx context.Context, limit int) ([]*entity.AttendanceEvent, error) {
	if limit <= 0 {
		limit = 100
	}
	return uc.attendanceRepo.ListEvents(ctx, limit)
}

// ProcessEvent processes an incoming attendance event and updates/creates attendance pairs
func (uc *AttendanceUseCase) ProcessEvent(ctx context.Context, event *entity.AttendanceEvent) error {
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
		// If event is OUT or AUTO, check if there is an existing pair today for this user
		// To support First-In, Last-Out logic (re-scanning OUT updates checkout time)
		if event.Direction == "OUT" || event.Direction == "AUTO" {
			if lastPair, err := uc.attendanceRepo.FindLastPairByUserAndDate(ctx, event.UserID, workDate); err == nil && lastPair != nil {
				if event.Direction == "OUT" || (event.Direction == "AUTO" && lastPair.CheckInEventID != nil) {
					now := time.Now().UTC()
					lastPair.UpdatedAt = now
					lastPair.CheckOutEventID = &event.EventID
					lastPair.CheckOutTime = &event.CreatedAt

					if lastPair.CheckInTime != nil {
						duration := event.CreatedAt.Sub(*lastPair.CheckInTime)
						hours := duration.Hours()
						if hours < 0 {
							hours = 0
						}
						lastPair.HoursWorked = &hours
					}
					lastPair.Status = CalculateAttendanceStatus(lastPair.CheckInTime, lastPair.CheckOutTime, lastPair.HoursWorked, lastPair.IsManual)
					return uc.attendanceRepo.UpdatePair(ctx, lastPair)
				}
			}
		}

		newPair := &entity.AttendancePair{
			UserID:    event.UserID,
			WorkDate:  workDate,
			CreatedAt: time.Now().UTC(),
			UpdatedAt: time.Now().UTC(),
		}

		if event.Direction == "OUT" {
			newPair.CheckOutEventID = &event.EventID
			newPair.CheckOutTime = &event.CreatedAt
			newPair.Status = CalculateAttendanceStatus(newPair.CheckInTime, newPair.CheckOutTime, nil, false)
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

		if pair.CheckInTime != nil {
			duration := event.CreatedAt.Sub(*pair.CheckInTime)
			hours := duration.Hours()
			if hours < 0 {
				hours = 0
			}
			pair.HoursWorked = &hours
		}
		pair.Status = CalculateAttendanceStatus(pair.CheckInTime, pair.CheckOutTime, pair.HoursWorked, pair.IsManual)
	} else if pair.CheckInEventID == nil {
		pair.CheckInEventID = &event.EventID
		pair.CheckInTime = &event.CreatedAt
		pair.Status = "open"
	}

	return uc.attendanceRepo.UpdatePair(ctx, pair)
}

// ManualAddAttendance creates a manual attendance record with audited reason
func (uc *AttendanceUseCase) ManualAddAttendance(ctx context.Context, adminID string, dto *ManualAttendanceDTO) (*entity.AttendancePair, error) {
	if dto.Reason == "" {
		return nil, errors.New("จำเป็นต้องระบุเหตุผลในการบันทึกเวลาแทน")
	}

	inTime, err := parseDateTime(dto.WorkDate, dto.CheckInTime)
	if err != nil {
		return nil, err
	}
	outTime, err := parseDateTime(dto.WorkDate, dto.CheckOutTime)
	if err != nil {
		return nil, err
	}

	var hoursWorked *float64
	if inTime != nil && outTime != nil {
		hrs := outTime.Sub(*inTime).Hours()
		if hrs < 0 {
			hrs = 0
		}
		hoursWorked = &hrs
	}

	now := time.Now().UTC()
	pair := &entity.AttendancePair{
		UserID:           dto.UserID,
		WorkDate:         dto.WorkDate,
		CheckInTime:      inTime,
		CheckOutTime:     outTime,
		HoursWorked:      hoursWorked,
		Status:           "adjusted",
		IsManual:         true,
		AdjustmentReason: dto.Reason,
		CreatedAt:        now,
		UpdatedAt:        now,
	}

	if inTime != nil {
		inEvID := fmt.Sprintf("manual-in-%s-%d", dto.UserID, now.UnixNano())
		pair.CheckInEventID = &inEvID
		_ = uc.attendanceRepo.SaveEvent(ctx, &entity.AttendanceEvent{
			EventID:     inEvID,
			UserID:      dto.UserID,
			DeviceID:    "manual-admin",
			Direction:   "IN",
			Confidence:  1.0,
			ClockSynced: true,
			CreatedAt:   *inTime,
			SyncedAt:    now,
		})
	}
	if outTime != nil {
		outEvID := fmt.Sprintf("manual-out-%s-%d", dto.UserID, now.UnixNano())
		pair.CheckOutEventID = &outEvID
		_ = uc.attendanceRepo.SaveEvent(ctx, &entity.AttendanceEvent{
			EventID:     outEvID,
			UserID:      dto.UserID,
			DeviceID:    "manual-admin",
			Direction:   "OUT",
			Confidence:  1.0,
			ClockSynced: true,
			CreatedAt:   *outTime,
			SyncedAt:    now,
		})
	}

	if err := uc.attendanceRepo.SavePair(ctx, pair); err != nil {
		return nil, err
	}

	if uc.auditRepo != nil {
		detailBytes, _ := json.Marshal(map[string]interface{}{
			"action":    "manual_add",
			"user_id":   dto.UserID,
			"work_date": dto.WorkDate,
			"reason":    dto.Reason,
			"check_in":  dto.CheckInTime,
			"check_out": dto.CheckOutTime,
		})
		var aID *string
		if adminID != "" {
			aID = &adminID
		}
		_ = uc.auditRepo.Log(ctx, &entity.AuditLog{
			AdminID:    aID,
			Action:     "attendance:manual_add",
			TargetType: "attendance_pair",
			TargetID:   fmt.Sprintf("%d", pair.ID),
			Detail:     string(detailBytes),
			CreatedAt:  now,
		})
	}

	return pair, nil
}

// ManualEditAttendance updates an existing attendance record with audited reason
func (uc *AttendanceUseCase) ManualEditAttendance(ctx context.Context, adminID string, pairID int, dto *UpdateAttendancePairDTO) (*entity.AttendancePair, error) {
	if dto.Reason == "" {
		return nil, errors.New("จำเป็นต้องระบุเหตุผลในการแก้ไขรายการลงเวลา")
	}

	pair, err := uc.attendanceRepo.GetPairByID(ctx, pairID)
	if err != nil {
		return nil, err
	}

	now := time.Now().UTC()
	oldDetail, _ := json.Marshal(map[string]interface{}{
		"check_in_time":  pair.CheckInTime,
		"check_out_time": pair.CheckOutTime,
		"hours_worked":   pair.HoursWorked,
		"status":         pair.Status,
	})

	inTime, err := parseDateTime(pair.WorkDate, dto.CheckInTime)
	if err != nil {
		return nil, err
	}
	outTime, err := parseDateTime(pair.WorkDate, dto.CheckOutTime)
	if err != nil {
		return nil, err
	}

	if inTime != nil {
		pair.CheckInTime = inTime
		if pair.CheckInEventID == nil {
			evID := fmt.Sprintf("manual-in-%s-%d", pair.UserID, now.UnixNano())
			pair.CheckInEventID = &evID
			_ = uc.attendanceRepo.SaveEvent(ctx, &entity.AttendanceEvent{
				EventID:     evID,
				UserID:      pair.UserID,
				DeviceID:    "manual-admin",
				Direction:   "IN",
				Confidence:  1.0,
				ClockSynced: true,
				CreatedAt:   *inTime,
				SyncedAt:    now,
			})
		}
	}
	if outTime != nil {
		pair.CheckOutTime = outTime
		if pair.CheckOutEventID == nil {
			evID := fmt.Sprintf("manual-out-%s-%d", pair.UserID, now.UnixNano())
			pair.CheckOutEventID = &evID
			_ = uc.attendanceRepo.SaveEvent(ctx, &entity.AttendanceEvent{
				EventID:     evID,
				UserID:      pair.UserID,
				DeviceID:    "manual-admin",
				Direction:   "OUT",
				Confidence:  1.0,
				ClockSynced: true,
				CreatedAt:   *outTime,
				SyncedAt:    now,
			})
		}
	}

	if pair.CheckInTime != nil && pair.CheckOutTime != nil {
		hrs := pair.CheckOutTime.Sub(*pair.CheckInTime).Hours()
		if hrs < 0 {
			hrs = 0
		}
		pair.HoursWorked = &hrs
	}

	pair.IsManual = true
	pair.AdjustmentReason = dto.Reason
	pair.Status = "adjusted"
	pair.UpdatedAt = now

	if err := uc.attendanceRepo.UpdatePair(ctx, pair); err != nil {
		return nil, err
	}

	if uc.auditRepo != nil {
		detailBytes, _ := json.Marshal(map[string]interface{}{
			"action":    "manual_edit",
			"pair_id":   pair.ID,
			"user_id":   pair.UserID,
			"reason":    dto.Reason,
			"old_state": string(oldDetail),
			"check_in":  dto.CheckInTime,
			"check_out": dto.CheckOutTime,
		})
		var aID *string
		if adminID != "" {
			aID = &adminID
		}
		_ = uc.auditRepo.Log(ctx, &entity.AuditLog{
			AdminID:    aID,
			Action:     "attendance:manual_edit",
			TargetType: "attendance_pair",
			TargetID:   fmt.Sprintf("%d", pair.ID),
			Detail:     string(detailBytes),
			CreatedAt:  now,
		})
	}

	return pair, nil
}
