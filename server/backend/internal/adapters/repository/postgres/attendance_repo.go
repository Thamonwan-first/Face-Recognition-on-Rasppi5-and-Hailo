package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"attendance-backend/internal/entity"
	"attendance-backend/internal/ports"
)

type attendanceRepo struct {
	db *sql.DB
}

func NewAttendanceRepository(db *sql.DB) ports.AttendanceRepository {
	return &attendanceRepo{db: db}
}

func (r *attendanceRepo) SaveEvent(ctx context.Context, ev *entity.AttendanceEvent) error {
	query := `
	INSERT INTO attendance_events (event_id, user_id, device_id, direction, confidence, clock_synced, snapshot_url, created_at, synced_at)
	VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
	`
	_, err := r.db.ExecContext(ctx, query,
		ev.EventID, ev.UserID, ev.DeviceID, ev.Direction, ev.Confidence, ev.ClockSynced, ev.SnapshotURL, ev.CreatedAt, ev.SyncedAt)
	return err
}

func (r *attendanceRepo) GetEventByID(ctx context.Context, eventID string) (*entity.AttendanceEvent, error) {
	query := `
	SELECT event_id, user_id, device_id, direction, confidence, clock_synced, COALESCE(snapshot_url, ''), created_at, synced_at
	FROM attendance_events WHERE event_id = $1
	`
	row := r.db.QueryRowContext(ctx, query, eventID)
	var ev entity.AttendanceEvent
	if err := row.Scan(&ev.EventID, &ev.UserID, &ev.DeviceID, &ev.Direction, &ev.Confidence, &ev.ClockSynced, &ev.SnapshotURL, &ev.CreatedAt, &ev.SyncedAt); err != nil {
		if err == sql.ErrNoRows {
			return nil, entity.ErrNotFound
		}
		return nil, err
	}
	return &ev, nil
}

func (r *attendanceRepo) ListEvents(ctx context.Context, limit int) ([]*entity.AttendanceEvent, error) {
	query := `
	SELECT event_id, user_id, device_id, direction, confidence, clock_synced, COALESCE(snapshot_url, ''), created_at, synced_at
	FROM attendance_events
	ORDER BY created_at DESC LIMIT $1
	`
	rows, err := r.db.QueryContext(ctx, query, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var events []*entity.AttendanceEvent
	for rows.Next() {
		var ev entity.AttendanceEvent
		if err := rows.Scan(&ev.EventID, &ev.UserID, &ev.DeviceID, &ev.Direction, &ev.Confidence, &ev.ClockSynced, &ev.SnapshotURL, &ev.CreatedAt, &ev.SyncedAt); err != nil {
			return nil, err
		}
		events = append(events, &ev)
	}
	return events, nil
}

func (r *attendanceRepo) ListPairs(ctx context.Context, startDate, endDate, userID, department, deviceID string, limit int) ([]*entity.AttendancePair, error) {
	query := `
	SELECT p.id, p.user_id, u.name, u.employee_id, u.department,
	       p.check_in_event_id, p.check_out_event_id,
	       in_ev.device_id as check_in_device_id,
	       out_ev.device_id as check_out_device_id,
	       in_ev.created_at as check_in_time,
	       out_ev.created_at as check_out_time,
	       COALESCE(in_ev.snapshot_url, p.check_in_snapshot, '') as in_snapshot,
	       COALESCE(out_ev.snapshot_url, p.check_out_snapshot, '') as out_snapshot,
	       p.work_date, p.hours_worked, p.status, p.is_manual, COALESCE(p.adjustment_reason, ''), p.created_at, p.updated_at
	FROM attendance_pairs p
	LEFT JOIN users u ON p.user_id = u.id
	LEFT JOIN attendance_events in_ev ON p.check_in_event_id = in_ev.event_id
	LEFT JOIN attendance_events out_ev ON p.check_out_event_id = out_ev.event_id
	WHERE 1=1
	`
	var args []any
	paramIdx := 1

	if startDate != "" {
		query += fmt.Sprintf(" AND p.work_date >= $%d", paramIdx)
		args = append(args, startDate)
		paramIdx++
	}
	if endDate != "" {
		query += fmt.Sprintf(" AND p.work_date <= $%d", paramIdx)
		args = append(args, endDate)
		paramIdx++
	}
	if userID != "" {
		query += fmt.Sprintf(" AND p.user_id = $%d", paramIdx)
		args = append(args, userID)
		paramIdx++
	}
	if department != "" {
		query += fmt.Sprintf(" AND u.department = $%d", paramIdx)
		args = append(args, department)
		paramIdx++
	}
	if deviceID != "" {
		query += fmt.Sprintf(" AND (in_ev.device_id = $%d OR out_ev.device_id = $%d)", paramIdx, paramIdx)
		args = append(args, deviceID)
		paramIdx++
	}

	query += fmt.Sprintf(" ORDER BY p.work_date DESC, p.created_at DESC LIMIT $%d", paramIdx)
	args = append(args, limit)

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var pairs []*entity.AttendancePair
	for rows.Next() {
		var p entity.AttendancePair
		var uName, empID, dept sql.NullString
		var inID, outID, inDev, outDev sql.NullString
		var inTime, outTime sql.NullTime
		var inSnap, outSnap sql.NullString
		var hours sql.NullFloat64

		if err := rows.Scan(
			&p.ID, &p.UserID, &uName, &empID, &dept,
			&inID, &outID, &inDev, &outDev, &inTime, &outTime, &inSnap, &outSnap,
			&p.WorkDate, &hours, &p.Status, &p.IsManual, &p.AdjustmentReason, &p.CreatedAt, &p.UpdatedAt,
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
		if inDev.Valid {
			p.CheckInDeviceID = &inDev.String
		}
		if outDev.Valid {
			p.CheckOutDeviceID = &outDev.String
		}
		if inTime.Valid {
			p.CheckInTime = &inTime.Time
		}
		if outTime.Valid {
			p.CheckOutTime = &outTime.Time
		}
		if inSnap.Valid && inSnap.String != "" {
			p.CheckInSnapshot = &inSnap.String
		}
		if outSnap.Valid && outSnap.String != "" {
			p.CheckOutSnapshot = &outSnap.String
		}
		if hours.Valid {
			p.HoursWorked = &hours.Float64
		}

		pairs = append(pairs, &p)
	}
	return pairs, nil
}

func (r *attendanceRepo) GetPairByID(ctx context.Context, id int) (*entity.AttendancePair, error) {
	query := `
	SELECT p.id, p.user_id, u.name, u.employee_id, u.department,
	       p.check_in_event_id, p.check_out_event_id,
	       in_ev.device_id as check_in_device_id,
	       out_ev.device_id as check_out_device_id,
	       in_ev.created_at as check_in_time,
	       out_ev.created_at as check_out_time,
	       COALESCE(in_ev.snapshot_url, p.check_in_snapshot, '') as in_snapshot,
	       COALESCE(out_ev.snapshot_url, p.check_out_snapshot, '') as out_snapshot,
	       p.work_date, p.hours_worked, p.status, p.is_manual, COALESCE(p.adjustment_reason, ''), p.created_at, p.updated_at
	FROM attendance_pairs p
	LEFT JOIN users u ON p.user_id = u.id
	LEFT JOIN attendance_events in_ev ON p.check_in_event_id = in_ev.event_id
	LEFT JOIN attendance_events out_ev ON p.check_out_event_id = out_ev.event_id
	WHERE p.id = $1
	`
	row := r.db.QueryRowContext(ctx, query, id)
	var p entity.AttendancePair
	var uName, empID, dept sql.NullString
	var inID, outID, inDev, outDev sql.NullString
	var inTime, outTime sql.NullTime
	var inSnap, outSnap sql.NullString
	var hours sql.NullFloat64

	if err := row.Scan(
		&p.ID, &p.UserID, &uName, &empID, &dept,
		&inID, &outID, &inDev, &outDev, &inTime, &outTime, &inSnap, &outSnap,
		&p.WorkDate, &hours, &p.Status, &p.IsManual, &p.AdjustmentReason, &p.CreatedAt, &p.UpdatedAt,
	); err != nil {
		if err == sql.ErrNoRows {
			return nil, entity.ErrNotFound
		}
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
	if inDev.Valid {
		p.CheckInDeviceID = &inDev.String
	}
	if outDev.Valid {
		p.CheckOutDeviceID = &outDev.String
	}
	if inTime.Valid {
		p.CheckInTime = &inTime.Time
	}
	if outTime.Valid {
		p.CheckOutTime = &outTime.Time
	}
	if inSnap.Valid && inSnap.String != "" {
		p.CheckInSnapshot = &inSnap.String
	}
	if outSnap.Valid && outSnap.String != "" {
		p.CheckOutSnapshot = &outSnap.String
	}
	if hours.Valid {
		p.HoursWorked = &hours.Float64
	}

	return &p, nil
}

func (r *attendanceRepo) FindOpenPair(ctx context.Context, userID, workDate string) (*entity.AttendancePair, error) {
	query := `
	SELECT p.id, p.user_id, p.check_in_event_id, p.check_out_event_id,
	       in_ev.created_at as check_in_time,
	       out_ev.created_at as check_out_time,
	       COALESCE(in_ev.snapshot_url, p.check_in_snapshot, '') as in_snapshot,
	       COALESCE(out_ev.snapshot_url, p.check_out_snapshot, '') as out_snapshot,
	       p.work_date, p.hours_worked, p.status, p.is_manual, COALESCE(p.adjustment_reason, ''), p.created_at, p.updated_at
	FROM attendance_pairs p
	LEFT JOIN attendance_events in_ev ON p.check_in_event_id = in_ev.event_id
	LEFT JOIN attendance_events out_ev ON p.check_out_event_id = out_ev.event_id
	WHERE p.user_id = $1 AND p.work_date = $2 AND p.status = 'open'
	ORDER BY p.created_at DESC LIMIT 1
	`
	row := r.db.QueryRowContext(ctx, query, userID, workDate)
	var p entity.AttendancePair
	var inID, outID sql.NullString
	var inTime, outTime sql.NullTime
	var inSnap, outSnap sql.NullString
	var hours sql.NullFloat64

	if err := row.Scan(&p.ID, &p.UserID, &inID, &outID, &inTime, &outTime, &inSnap, &outSnap, &p.WorkDate, &hours, &p.Status, &p.IsManual, &p.AdjustmentReason, &p.CreatedAt, &p.UpdatedAt); err != nil {
		if err == sql.ErrNoRows {
			return nil, entity.ErrNotFound
		}
		return nil, err
	}

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
	if inSnap.Valid && inSnap.String != "" {
		p.CheckInSnapshot = &inSnap.String
	}
	if outSnap.Valid && outSnap.String != "" {
		p.CheckOutSnapshot = &outSnap.String
	}
	if hours.Valid {
		p.HoursWorked = &hours.Float64
	}

	return &p, nil
}

func (r *attendanceRepo) FindLastPairByUserAndDate(ctx context.Context, userID, workDate string) (*entity.AttendancePair, error) {
	query := `
	SELECT p.id, p.user_id, p.check_in_event_id, p.check_out_event_id,
	       in_ev.created_at as check_in_time,
	       out_ev.created_at as check_out_time,
	       COALESCE(in_ev.snapshot_url, p.check_in_snapshot, '') as in_snapshot,
	       COALESCE(out_ev.snapshot_url, p.check_out_snapshot, '') as out_snapshot,
	       p.work_date, p.hours_worked, p.status, p.is_manual, COALESCE(p.adjustment_reason, ''), p.created_at, p.updated_at
	FROM attendance_pairs p
	LEFT JOIN attendance_events in_ev ON p.check_in_event_id = in_ev.event_id
	LEFT JOIN attendance_events out_ev ON p.check_out_event_id = out_ev.event_id
	WHERE p.user_id = $1 AND p.work_date = $2
	ORDER BY (CASE WHEN p.check_in_event_id IS NOT NULL THEN 0 ELSE 1 END), p.id DESC
	LIMIT 1
	`
	row := r.db.QueryRowContext(ctx, query, userID, workDate)
	var p entity.AttendancePair
	var inID, outID sql.NullString
	var inTime, outTime sql.NullTime
	var inSnap, outSnap sql.NullString
	var hours sql.NullFloat64

	if err := row.Scan(&p.ID, &p.UserID, &inID, &outID, &inTime, &outTime, &inSnap, &outSnap, &p.WorkDate, &hours, &p.Status, &p.IsManual, &p.AdjustmentReason, &p.CreatedAt, &p.UpdatedAt); err != nil {
		if err == sql.ErrNoRows {
			return nil, entity.ErrNotFound
		}
		return nil, err
	}

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
	if inSnap.Valid && inSnap.String != "" {
		p.CheckInSnapshot = &inSnap.String
	}
	if outSnap.Valid && outSnap.String != "" {
		p.CheckOutSnapshot = &outSnap.String
	}
	if hours.Valid {
		p.HoursWorked = &hours.Float64
	}

	return &p, nil
}

func (r *attendanceRepo) SavePair(ctx context.Context, p *entity.AttendancePair) error {
	query := `
	INSERT INTO attendance_pairs (user_id, check_in_event_id, check_out_event_id, check_in_snapshot, check_out_snapshot, work_date, hours_worked, status, is_manual, adjustment_reason, created_at, updated_at)
	VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
	RETURNING id
	`
	err := r.db.QueryRowContext(ctx, query,
		p.UserID, p.CheckInEventID, p.CheckOutEventID, p.CheckInSnapshot, p.CheckOutSnapshot, p.WorkDate, p.HoursWorked, p.Status, p.IsManual, p.AdjustmentReason, p.CreatedAt, p.UpdatedAt).Scan(&p.ID)
	return err
}

func (r *attendanceRepo) UpdatePair(ctx context.Context, p *entity.AttendancePair) error {
	query := `
	UPDATE attendance_pairs SET
		check_in_event_id = $1, check_out_event_id = $2, check_in_snapshot = $3, check_out_snapshot = $4, hours_worked = $5, status = $6, is_manual = $7, adjustment_reason = $8, updated_at = $9
	WHERE id = $10
	`
	_, err := r.db.ExecContext(ctx, query,
		p.CheckInEventID, p.CheckOutEventID, p.CheckInSnapshot, p.CheckOutSnapshot, p.HoursWorked, p.Status, p.IsManual, p.AdjustmentReason, p.UpdatedAt, p.ID)
	return err
}

func (r *attendanceRepo) SaveEventAndPair(ctx context.Context, event *entity.AttendanceEvent, pair *entity.AttendancePair, isUpdate bool) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// 1. Save event
	queryEv := `
	INSERT INTO attendance_events (event_id, user_id, device_id, direction, confidence, clock_synced, snapshot_url, created_at, synced_at)
	VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
	`
	if _, err := tx.ExecContext(ctx, queryEv,
		event.EventID, event.UserID, event.DeviceID, event.Direction, event.Confidence, event.ClockSynced, event.SnapshotURL, event.CreatedAt, event.SyncedAt); err != nil {
		return err
	}

	// 2. Save or update pair atomically
	if pair != nil {
		if isUpdate {
			queryPair := `
			UPDATE attendance_pairs SET
				check_in_event_id = $1, check_out_event_id = $2, check_in_snapshot = $3, check_out_snapshot = $4, hours_worked = $5, status = $6, is_manual = $7, adjustment_reason = $8, updated_at = $9
			WHERE id = $10
			`
			if _, err := tx.ExecContext(ctx, queryPair,
				pair.CheckInEventID, pair.CheckOutEventID, pair.CheckInSnapshot, pair.CheckOutSnapshot, pair.HoursWorked, pair.Status, pair.IsManual, pair.AdjustmentReason, pair.UpdatedAt, pair.ID); err != nil {
				return err
			}
		} else {
			queryPair := `
			INSERT INTO attendance_pairs (user_id, check_in_event_id, check_out_event_id, check_in_snapshot, check_out_snapshot, work_date, hours_worked, status, is_manual, adjustment_reason, created_at, updated_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
			RETURNING id
			`
			if err := tx.QueryRowContext(ctx, queryPair,
				pair.UserID, pair.CheckInEventID, pair.CheckOutEventID, pair.CheckInSnapshot, pair.CheckOutSnapshot, pair.WorkDate, pair.HoursWorked, pair.Status, pair.IsManual, pair.AdjustmentReason, pair.CreatedAt, pair.UpdatedAt).Scan(&pair.ID); err != nil {
				return err
			}
		}
	}

	return tx.Commit()
}

func (r *attendanceRepo) SaveUnrecognized(ctx context.Context, scan *entity.UnrecognizedScan) error {
	query := `
	INSERT INTO unrecognized_scans (id, device_id, snapshot_url, confidence, status, created_at)
	VALUES ($1, $2, $3, $4, $5, $6)
	`
	_, err := r.db.ExecContext(ctx, query, scan.ID, scan.DeviceID, scan.SnapshotURL, scan.Confidence, scan.Status, scan.CreatedAt)
	return err
}

func (r *attendanceRepo) ListUnrecognized(ctx context.Context, limit int) ([]*entity.UnrecognizedScan, error) {
	if limit <= 0 {
		limit = 100
	}
	query := `
	SELECT id, device_id, snapshot_url, confidence, status, resolved_user_id, resolved_at, created_at
	FROM unrecognized_scans
	WHERE status = 'pending'
	ORDER BY created_at DESC LIMIT $1
	`
	rows, err := r.db.QueryContext(ctx, query, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []*entity.UnrecognizedScan
	for rows.Next() {
		var s entity.UnrecognizedScan
		var resUser sql.NullString
		var resAt sql.NullTime
		if err := rows.Scan(&s.ID, &s.DeviceID, &s.SnapshotURL, &s.Confidence, &s.Status, &resUser, &resAt, &s.CreatedAt); err != nil {
			return nil, err
		}
		if resUser.Valid {
			s.ResolvedUserID = &resUser.String
		}
		if resAt.Valid {
			s.ResolvedAt = &resAt.Time
		}
		list = append(list, &s)
	}
	return list, nil
}

func (r *attendanceRepo) GetUnrecognizedByID(ctx context.Context, id string) (*entity.UnrecognizedScan, error) {
	query := `
	SELECT id, device_id, snapshot_url, confidence, status, resolved_user_id, resolved_at, created_at
	FROM unrecognized_scans
	WHERE id = $1
	`
	var s entity.UnrecognizedScan
	var resUser sql.NullString
	var resAt sql.NullTime
	row := r.db.QueryRowContext(ctx, query, id)
	if err := row.Scan(&s.ID, &s.DeviceID, &s.SnapshotURL, &s.Confidence, &s.Status, &resUser, &resAt, &s.CreatedAt); err != nil {
		if err == sql.ErrNoRows {
			return nil, entity.ErrNotFound
		}
		return nil, err
	}
	if resUser.Valid {
		s.ResolvedUserID = &resUser.String
	}
	if resAt.Valid {
		s.ResolvedAt = &resAt.Time
	}
	return &s, nil
}

func (r *attendanceRepo) ResolveUnrecognized(ctx context.Context, id, userID, status string) error {
	now := time.Now().UTC()
	query := `
	UPDATE unrecognized_scans
	SET status = $1, resolved_user_id = $2, resolved_at = $3
	WHERE id = $4
	`
	_, err := r.db.ExecContext(ctx, query, status, userID, now, id)
	return err
}

func (r *attendanceRepo) DeleteUnrecognized(ctx context.Context, id string) error {
	_, err := r.db.ExecContext(ctx, "DELETE FROM unrecognized_scans WHERE id = $1", id)
	return err
}
