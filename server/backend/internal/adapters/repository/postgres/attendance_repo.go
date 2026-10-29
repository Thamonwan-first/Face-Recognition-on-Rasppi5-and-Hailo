package postgres

import (
	"context"
	"database/sql"
	"time"

	"attendance-backend/internal/core/domain"
	"attendance-backend/internal/core/ports"
)

type attendanceRepo struct {
	db *sql.DB
}

func NewAttendanceRepository(db *sql.DB) ports.AttendanceRepository {
	return &attendanceRepo{db: db}
}

func (r *attendanceRepo) SaveEvent(ctx context.Context, ev *domain.AttendanceEvent) error {
	query := `
	INSERT INTO attendance_events (event_id, user_id, device_id, direction, confidence, clock_synced, created_at, synced_at)
	VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`
	_, err := r.db.ExecContext(ctx, query,
		ev.EventID, ev.UserID, ev.DeviceID, ev.Direction, ev.Confidence, ev.ClockSynced, ev.CreatedAt, ev.SyncedAt)
	return err
}

func (r *attendanceRepo) GetEventByID(ctx context.Context, eventID string) (*domain.AttendanceEvent, error) {
	query := `
	SELECT event_id, user_id, device_id, direction, confidence, clock_synced, created_at, synced_at
	FROM attendance_events WHERE event_id = $1
	`
	row := r.db.QueryRowContext(ctx, query, eventID)
	var ev domain.AttendanceEvent
	if err := row.Scan(&ev.EventID, &ev.UserID, &ev.DeviceID, &ev.Direction, &ev.Confidence, &ev.ClockSynced, &ev.CreatedAt, &ev.SyncedAt); err != nil {
		if err == sql.ErrNoRows {
			return nil, domain.ErrNotFound
		}
		return nil, err
	}
	return &ev, nil
}

func (r *attendanceRepo) ListEvents(ctx context.Context, limit int) ([]*domain.AttendanceEvent, error) {
	query := `
	SELECT event_id, user_id, device_id, direction, confidence, clock_synced, created_at, synced_at
	FROM attendance_events
	ORDER BY created_at DESC LIMIT $1
	`
	rows, err := r.db.QueryContext(ctx, query, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var events []*domain.AttendanceEvent
	for rows.Next() {
		var ev domain.AttendanceEvent
		if err := rows.Scan(&ev.EventID, &ev.UserID, &ev.DeviceID, &ev.Direction, &ev.Confidence, &ev.ClockSynced, &ev.CreatedAt, &ev.SyncedAt); err != nil {
			return nil, err
		}
		events = append(events, &ev)
	}
	return events, nil
}

func (r *attendanceRepo) ListPairs(ctx context.Context, startDate, endDate, userID, department string, limit int) ([]*domain.AttendancePair, error) {
	query := `
	SELECT p.id, p.user_id, u.name, u.employee_id, u.department,
	       p.check_in_event_id, p.check_out_event_id,
	       in_ev.created_at as check_in_time,
	       out_ev.created_at as check_out_time,
	       p.work_date, p.hours_worked, p.status, p.created_at, p.updated_at
	FROM attendance_pairs p
	LEFT JOIN users u ON p.user_id = u.id
	LEFT JOIN attendance_events in_ev ON p.check_in_event_id = in_ev.event_id
	LEFT JOIN attendance_events out_ev ON p.check_out_event_id = out_ev.event_id
	WHERE 1=1
	`
	var args []any
	paramIdx := 1

	if startDate != "" {
		query += fmtSprintf(" AND p.work_date >= $%d", paramIdx)
		args = append(args, startDate)
		paramIdx++
	}
	if endDate != "" {
		query += fmtSprintf(" AND p.work_date <= $%d", paramIdx)
		args = append(args, endDate)
		paramIdx++
	}
	if userID != "" {
		query += fmtSprintf(" AND p.user_id = $%d", paramIdx)
		args = append(args, userID)
		paramIdx++
	}
	if department != "" {
		query += fmtSprintf(" AND u.department = $%d", paramIdx)
		args = append(args, department)
		paramIdx++
	}

	query += fmtSprintf(" ORDER BY p.work_date DESC, p.created_at DESC LIMIT $%d", paramIdx)
	args = append(args, limit)

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var pairs []*domain.AttendancePair
	for rows.Next() {
		var p domain.AttendancePair
		var uName, empID, dept sql.NullString
		var inID, outID sql.NullString
		var inTime, outTime sql.NullTime
		var hours sql.NullFloat64

		if err := rows.Scan(
			&p.ID, &p.UserID, &uName, &empID, &dept,
			&inID, &outID, &inTime, &outTime,
			&p.WorkDate, &hours, &p.Status, &p.CreatedAt, &p.UpdatedAt,
		); err != nil {
			return nil, err
		}

		p.UserName = uName.String
		p.EmployeeID = empID.String
		p.Department = dept.String

		if inID.Valid {
			p.CheckInEventID = &inID.String
		}
		if outID.Valid {
			p.CheckOutEventID = &outID.String
		}
		if inTime.Valid {
			p.CheckInTime = &inTime.Time
		}
		if outTime.Valid {
			p.CheckOutTime = &outTime.Time
		}
		if hours.Valid {
			p.HoursWorked = &hours.Float64
		}

		pairs = append(pairs, &p)
	}
	return pairs, nil
}

func fmtSprintf(format string, a ...any) string {
	var res string
	for _, v := range a {
		res += format
		_ = v
	}
	// Simple wrapper for formatting query string with placeholder
	return fmtSprintfSimple(format, a...)
}

func fmtSprintfSimple(format string, a ...any) string {
	if len(a) == 1 {
		switch v := a[0].(type) {
		case int:
			var out string
			for i := 0; i < len(format); i++ {
				if i < len(format)-2 && format[i] == '%' && format[i+1] == 'd' {
					out += itoa(v)
					i++
				} else {
					out += string(format[i])
				}
			}
			return out
		}
	}
	return format
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var res []byte
	for n > 0 {
		res = append([]byte{byte('0' + (n % 10))}, res...)
		n /= 10
	}
	return string(res)
}

func (r *attendanceRepo) FindOpenPair(ctx context.Context, userID, workDate string) (*domain.AttendancePair, error) {
	query := `
	SELECT id, user_id, check_in_event_id, check_out_event_id, work_date, hours_worked, status, created_at, updated_at
	FROM attendance_pairs
	WHERE user_id = $1 AND work_date = $2 AND status = 'open'
	ORDER BY created_at DESC LIMIT 1
	`
	row := r.db.QueryRowContext(ctx, query, userID, workDate)
	var p domain.AttendancePair
	var inID, outID sql.NullString
	var hours sql.NullFloat64

	if err := row.Scan(&p.ID, &p.UserID, &inID, &outID, &p.WorkDate, &hours, &p.Status, &p.CreatedAt, &p.UpdatedAt); err != nil {
		if err == sql.ErrNoRows {
			return nil, domain.ErrNotFound
		}
		return nil, err
	}

	if inID.Valid {
		p.CheckInEventID = &inID.String
		var t time.Time
		if err := r.db.QueryRowContext(ctx, "SELECT created_at FROM attendance_events WHERE event_id = $1", inID.String).Scan(&t); err == nil {
			p.CheckInTime = &t
		}
	}
	if outID.Valid {
		p.CheckOutEventID = &outID.String
		var t time.Time
		if err := r.db.QueryRowContext(ctx, "SELECT created_at FROM attendance_events WHERE event_id = $1", outID.String).Scan(&t); err == nil {
			p.CheckOutTime = &t
		}
	}
	if hours.Valid {
		p.HoursWorked = &hours.Float64
	}

	return &p, nil
}

func (r *attendanceRepo) SavePair(ctx context.Context, p *domain.AttendancePair) error {
	query := `
	INSERT INTO attendance_pairs (user_id, check_in_event_id, check_out_event_id, work_date, hours_worked, status, created_at, updated_at)
	VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	RETURNING id
	`
	err := r.db.QueryRowContext(ctx, query,
		p.UserID, p.CheckInEventID, p.CheckOutEventID, p.WorkDate, p.HoursWorked, p.Status, p.CreatedAt, p.UpdatedAt).Scan(&p.ID)
	return err
}

func (r *attendanceRepo) UpdatePair(ctx context.Context, p *domain.AttendancePair) error {
	query := `
	UPDATE attendance_pairs SET
		check_in_event_id = $1, check_out_event_id = $2, hours_worked = $3, status = $4, updated_at = $5
	WHERE id = $6
	`
	_, err := r.db.ExecContext(ctx, query,
		p.CheckInEventID, p.CheckOutEventID, p.HoursWorked, p.Status, p.UpdatedAt, p.ID)
	return err
}
