package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"attendance-backend/internal/entity"
	"attendance-backend/internal/ports"
)

type meetingRepo struct {
	db *sql.DB
}

func NewMeetingRepository(db *sql.DB) ports.MeetingRepository {
	return &meetingRepo{db: db}
}

func (r *meetingRepo) Create(ctx context.Context, m *entity.Meeting) error {
	query := `
	INSERT INTO meetings (id, title, description, location, device_id, meeting_date, start_time, end_time, late_threshold_minutes, status, created_by, created_at, updated_at)
	VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
	`
	_, err := r.db.ExecContext(ctx, query,
		m.ID, m.Title, m.Description, m.Location, m.DeviceID, m.MeetingDate, m.StartTime, m.EndTime, m.LateThresholdMinutes, m.Status, m.CreatedBy, m.CreatedAt, m.UpdatedAt)
	return err
}

func (r *meetingRepo) Update(ctx context.Context, m *entity.Meeting) error {
	query := `
	UPDATE meetings SET
		title = $1, description = $2, location = $3, device_id = $4,
		meeting_date = $5, start_time = $6, end_time = $7, late_threshold_minutes = $8,
		status = $9, updated_at = $10
	WHERE id = $11
	`
	_, err := r.db.ExecContext(ctx, query,
		m.Title, m.Description, m.Location, m.DeviceID,
		m.MeetingDate, m.StartTime, m.EndTime, m.LateThresholdMinutes,
		m.Status, m.UpdatedAt, m.ID)
	return err
}

func (r *meetingRepo) GetByID(ctx context.Context, id string) (*entity.Meeting, error) {
	query := `
	SELECT m.id, m.title, m.description, m.location, m.device_id, COALESCE(d.name, ''),
	       m.meeting_date, m.start_time, m.end_time, m.late_threshold_minutes, m.status,
	       m.created_by, m.created_at, m.updated_at,
	       COUNT(a.id) as attendee_count
	FROM meetings m
	LEFT JOIN devices d ON m.device_id = d.id
	LEFT JOIN meeting_attendees a ON m.id = a.meeting_id
	WHERE m.id = $1
	GROUP BY m.id, d.name
	`
	row := r.db.QueryRowContext(ctx, query, id)
	return scanMeetingRow(row)
}

func scanMeetingRow(row *sql.Row) (*entity.Meeting, error) {
	var m entity.Meeting
	var devID, devName sql.NullString
	var meetDate time.Time
	var count int

	if err := row.Scan(
		&m.ID, &m.Title, &m.Description, &m.Location, &devID, &devName,
		&meetDate, &m.StartTime, &m.EndTime, &m.LateThresholdMinutes, &m.Status,
		&m.CreatedBy, &m.CreatedAt, &m.UpdatedAt, &count,
	); err != nil {
		if err == sql.ErrNoRows {
			return nil, entity.ErrNotFound
		}
		return nil, err
	}
	if devID.Valid {
		m.DeviceID = &devID.String
	}
	if devName.Valid {
		m.DeviceName = devName.String
	}
	m.MeetingDate = meetDate.Format("2006-01-02")
	m.AttendeeCount = count
	return &m, nil
}

func (r *meetingRepo) List(ctx context.Context, date string, status string, deviceID string) ([]*entity.Meeting, error) {
	query := `
	SELECT m.id, m.title, m.description, m.location, m.device_id, COALESCE(d.name, ''),
	       m.meeting_date, m.start_time, m.end_time, m.late_threshold_minutes, m.status,
	       m.created_by, m.created_at, m.updated_at,
	       COUNT(a.id) as attendee_count
	FROM meetings m
	LEFT JOIN devices d ON m.device_id = d.id
	LEFT JOIN meeting_attendees a ON m.id = a.meeting_id
	WHERE 1=1
	`
	var args []any
	paramIdx := 1

	if date != "" {
		query += fmt.Sprintf(" AND m.meeting_date = $%d", paramIdx)
		args = append(args, date)
		paramIdx++
	}
	if status != "" {
		query += fmt.Sprintf(" AND m.status = $%d", paramIdx)
		args = append(args, status)
		paramIdx++
	}
	if deviceID != "" {
		query += fmt.Sprintf(" AND m.device_id = $%d", paramIdx)
		args = append(args, deviceID)
		paramIdx++
	}

	query += `
	GROUP BY m.id, d.name
	ORDER BY m.meeting_date DESC, m.start_time DESC
	`

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	meetings := make([]*entity.Meeting, 0)
	for rows.Next() {
		var m entity.Meeting
		var devID, devName sql.NullString
		var meetDate time.Time
		var count int

		if err := rows.Scan(
			&m.ID, &m.Title, &m.Description, &m.Location, &devID, &devName,
			&meetDate, &m.StartTime, &m.EndTime, &m.LateThresholdMinutes, &m.Status,
			&m.CreatedBy, &m.CreatedAt, &m.UpdatedAt, &count,
		); err != nil {
			return nil, err
		}
		if devID.Valid {
			m.DeviceID = &devID.String
		}
		if devName.Valid {
			m.DeviceName = devName.String
		}
		m.MeetingDate = meetDate.Format("2006-01-02")
		m.AttendeeCount = count
		meetings = append(meetings, &m)
	}
	return meetings, nil
}

func (r *meetingRepo) Delete(ctx context.Context, id string) error {
	_, err := r.db.ExecContext(ctx, "DELETE FROM meetings WHERE id = $1", id)
	return err
}

func (r *meetingRepo) FindActiveMeetingByDevice(ctx context.Context, deviceID string, date string, timeStr string) (*entity.Meeting, error) {
	// Active meeting: meeting_date matches, and current time is between start_time (minus 30 mins early check-in) and end_time
	// timeStr format HH:MM
	query := `
	SELECT m.id, m.title, m.description, m.location, m.device_id, COALESCE(d.name, ''),
	       m.meeting_date, m.start_time, m.end_time, m.late_threshold_minutes, m.status,
	       m.created_by, m.created_at, m.updated_at,
	       0 as attendee_count
	FROM meetings m
	LEFT JOIN devices d ON m.device_id = d.id
	WHERE m.device_id = $1 AND m.meeting_date = $2 AND m.status != 'cancelled'
	  AND (m.start_time <= $3 OR $3 >= '00:00') AND m.end_time >= $3
	ORDER BY m.start_time ASC
	LIMIT 1
	`
	row := r.db.QueryRowContext(ctx, query, deviceID, date, timeStr)
	return scanMeetingRow(row)
}

func (r *meetingRepo) RecordAttendee(ctx context.Context, a *entity.MeetingAttendee) error {
	query := `
	INSERT INTO meeting_attendees (meeting_id, user_id, event_id, scanned_at, snapshot_url, status, minutes_late, notes, created_at)
	VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
	ON CONFLICT (meeting_id, user_id) DO UPDATE SET
		scanned_at = EXCLUDED.scanned_at,
		snapshot_url = CASE WHEN EXCLUDED.snapshot_url != '' THEN EXCLUDED.snapshot_url ELSE meeting_attendees.snapshot_url END,
		status = EXCLUDED.status,
		minutes_late = EXCLUDED.minutes_late,
		notes = EXCLUDED.notes
	RETURNING id
	`
	return r.db.QueryRowContext(ctx, query,
		a.MeetingID, a.UserID, a.EventID, a.ScannedAt, a.SnapshotURL, a.Status, a.MinutesLate, a.Notes, a.CreatedAt).Scan(&a.ID)
}

func (r *meetingRepo) ListAttendees(ctx context.Context, meetingID string) ([]*entity.MeetingAttendee, error) {
	query := `
	SELECT a.id, a.meeting_id, a.user_id, COALESCE(u.name, 'Unknown'), COALESCE(u.employee_id, ''), COALESCE(u.department, ''),
	       COALESCE(a.event_id, ''), a.scanned_at, a.snapshot_url, a.status, a.minutes_late, COALESCE(a.notes, ''), a.created_at
	FROM meeting_attendees a
	LEFT JOIN users u ON a.user_id = u.id
	WHERE a.meeting_id = $1
	ORDER BY a.scanned_at ASC
	`
	rows, err := r.db.QueryContext(ctx, query, meetingID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	attendees := make([]*entity.MeetingAttendee, 0)
	for rows.Next() {
		var a entity.MeetingAttendee
		if err := rows.Scan(
			&a.ID, &a.MeetingID, &a.UserID, &a.UserName, &a.EmployeeID, &a.Department,
			&a.EventID, &a.ScannedAt, &a.SnapshotURL, &a.Status, &a.MinutesLate, &a.Notes, &a.CreatedAt,
		); err != nil {
			return nil, err
		}
		attendees = append(attendees, &a)
	}
	return attendees, nil
}

func (r *meetingRepo) GetAttendee(ctx context.Context, meetingID string, userID string) (*entity.MeetingAttendee, error) {
	query := `
	SELECT a.id, a.meeting_id, a.user_id, COALESCE(u.name, 'Unknown'), COALESCE(u.employee_id, ''), COALESCE(u.department, ''),
	       COALESCE(a.event_id, ''), a.scanned_at, a.snapshot_url, a.status, a.minutes_late, COALESCE(a.notes, ''), a.created_at
	FROM meeting_attendees a
	LEFT JOIN users u ON a.user_id = u.id
	WHERE a.meeting_id = $1 AND a.user_id = $2
	`
	row := r.db.QueryRowContext(ctx, query, meetingID, userID)
	var a entity.MeetingAttendee
	if err := row.Scan(
		&a.ID, &a.MeetingID, &a.UserID, &a.UserName, &a.EmployeeID, &a.Department,
		&a.EventID, &a.ScannedAt, &a.SnapshotURL, &a.Status, &a.MinutesLate, &a.Notes, &a.CreatedAt,
	); err != nil {
		if err == sql.ErrNoRows {
			return nil, entity.ErrNotFound
		}
		return nil, err
	}
	return &a, nil
}
