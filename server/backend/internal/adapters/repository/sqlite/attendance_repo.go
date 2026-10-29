package sqlite

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
	VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`
	_, err := r.db.ExecContext(ctx, query,
		ev.EventID, ev.UserID, ev.DeviceID, ev.Direction, ev.Confidence, ev.ClockSynced, ev.CreatedAt, ev.SyncedAt)
	return err
}

func (r *attendanceRepo) GetEventByID(ctx context.Context, eventID string) (*domain.AttendanceEvent, error) {
	query := `
	SELECT event_id, user_id, device_id, direction, confidence, clock_synced, created_at, synced_at
	FROM attendance_events WHERE event_id = ?
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
	ORDER BY created_at DESC LIMIT ?
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
	if startDate != "" {
		query += " AND p.work_date >= ?"
		args = append(args, startDate)
	}
	if endDate != "" {
		query += " AND p.work_date <= ?"
		args = append(args, endDate)
	}
	if userID != "" {
		query += " AND p.user_id = ?"
		args = append(args, userID)
	}
	if department != "" {
		query += " AND u.department = ?"
		args = append(args, department)
	}

	query += " ORDER BY p.work_date DESC, p.created_at DESC LIMIT ?"
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

func (r *attendanceRepo) FindOpenPair(ctx context.Context, userID, workDate string) (*domain.AttendancePair, error) {
	query := `
	SELECT id, user_id, check_in_event_id, check_out_event_id, work_date, hours_worked, status, created_at, updated_at
	FROM attendance_pairs
	WHERE user_id = ? AND work_date = ? AND status = 'open'
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
		// Load check-in time
		var t time.Time
		if err := r.db.QueryRowContext(ctx, "SELECT created_at FROM attendance_events WHERE event_id = ?", inID.String).Scan(&t); err == nil {
			p.CheckInTime = &t
		}
	}
	if outID.Valid {
		p.CheckOutEventID = &outID.String
		var t time.Time
		if err := r.db.QueryRowContext(ctx, "SELECT created_at FROM attendance_events WHERE event_id = ?", outID.String).Scan(&t); err == nil {
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
	VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`
	res, err := r.db.ExecContext(ctx, query,
		p.UserID, p.CheckInEventID, p.CheckOutEventID, p.WorkDate, p.HoursWorked, p.Status, p.CreatedAt, p.UpdatedAt)
	if err != nil {
		return err
	}
	id, err := res.LastInsertId()
	if err == nil {
		p.ID = int(id)
	}
	return nil
}

func (r *attendanceRepo) UpdatePair(ctx context.Context, p *domain.AttendancePair) error {
	query := `
	UPDATE attendance_pairs SET
		check_in_event_id = ?, check_out_event_id = ?, hours_worked = ?, status = ?, updated_at = ?
	WHERE id = ?
	`
	_, err := r.db.ExecContext(ctx, query,
		p.CheckInEventID, p.CheckOutEventID, p.HoursWorked, p.Status, p.UpdatedAt, p.ID)
	return err
}
