package sqlite

import (
	"context"
	"database/sql"
	"time"

	"attendance-backend/internal/core/domain"
	"attendance-backend/internal/core/ports"
)

type adminRepo struct {
	db *sql.DB
}

func NewAdminRepository(db *sql.DB) ports.AdminRepository {
	return &adminRepo{db: db}
}

func (r *adminRepo) GetByUsername(ctx context.Context, username string) (*domain.Admin, error) {
	row := r.db.QueryRowContext(ctx, "SELECT id, username, password_hash, role, created_at, last_login FROM admins WHERE username = ?", username)
	var a domain.Admin
	var lastLogin sql.NullTime
	if err := row.Scan(&a.ID, &a.Username, &a.PasswordHash, &a.Role, &a.CreatedAt, &lastLogin); err != nil {
		if err == sql.ErrNoRows {
			return nil, domain.ErrNotFound
		}
		return nil, err
	}
	if lastLogin.Valid {
		a.LastLogin = &lastLogin.Time
	}
	return &a, nil
}

func (r *adminRepo) GetByID(ctx context.Context, id string) (*domain.Admin, error) {
	row := r.db.QueryRowContext(ctx, "SELECT id, username, password_hash, role, created_at, last_login FROM admins WHERE id = ?", id)
	var a domain.Admin
	var lastLogin sql.NullTime
	if err := row.Scan(&a.ID, &a.Username, &a.PasswordHash, &a.Role, &a.CreatedAt, &lastLogin); err != nil {
		if err == sql.ErrNoRows {
			return nil, domain.ErrNotFound
		}
		return nil, err
	}
	if lastLogin.Valid {
		a.LastLogin = &lastLogin.Time
	}
	return &a, nil
}

func (r *adminRepo) Create(ctx context.Context, admin *domain.Admin) error {
	_, err := r.db.ExecContext(ctx, "INSERT INTO admins (id, username, password_hash, role, created_at) VALUES (?, ?, ?, ?, ?)",
		admin.ID, admin.Username, admin.PasswordHash, admin.Role, admin.CreatedAt)
	return err
}

func (r *adminRepo) UpdateLastLogin(ctx context.Context, id string, lastLogin time.Time) error {
	_, err := r.db.ExecContext(ctx, "UPDATE admins SET last_login = ? WHERE id = ?", lastLogin, id)
	return err
}

// ---------------- Device Repo ----------------

type deviceRepo struct {
	db *sql.DB
}

func NewDeviceRepository(db *sql.DB) ports.DeviceRepository {
	return &deviceRepo{db: db}
}

func (r *deviceRepo) GetByID(ctx context.Context, id string) (*domain.Device, error) {
	row := r.db.QueryRowContext(ctx, "SELECT id, name, location, direction, api_key_hash, status, last_seen, created_at, revoked_at FROM devices WHERE id = ?", id)
	return scanDeviceRow(row)
}

func (r *deviceRepo) GetByApiKeyHash(ctx context.Context, hash string) (*domain.Device, error) {
	row := r.db.QueryRowContext(ctx, "SELECT id, name, location, direction, api_key_hash, status, last_seen, created_at, revoked_at FROM devices WHERE api_key_hash = ? AND status != 'revoked'", hash)
	return scanDeviceRow(row)
}

func scanDeviceRow(row *sql.Row) (*domain.Device, error) {
	var d domain.Device
	var lastSeen, revokedAt sql.NullTime
	if err := row.Scan(&d.ID, &d.Name, &d.Location, &d.Direction, &d.ApiKeyHash, &d.Status, &lastSeen, &d.CreatedAt, &revokedAt); err != nil {
		if err == sql.ErrNoRows {
			return nil, domain.ErrNotFound
		}
		return nil, err
	}
	if lastSeen.Valid {
		d.LastSeen = &lastSeen.Time
	}
	if revokedAt.Valid {
		d.RevokedAt = &revokedAt.Time
	}
	return &d, nil
}

func (r *deviceRepo) List(ctx context.Context) ([]*domain.Device, error) {
	query := `
	SELECT d.id, d.name, d.location, d.direction, d.api_key_hash, d.status, d.last_seen, d.created_at, d.revoked_at,
	       s.applied_version, s.model_version, s.sw_version, s.cpu_temp, s.fps, s.outbox_pending, s.updated_at
	FROM devices d
	LEFT JOIN device_state s ON d.id = s.device_id
	ORDER BY d.created_at DESC
	`
	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var devices []*domain.Device
	for rows.Next() {
		var d domain.Device
		var lastSeen, revokedAt sql.NullTime
		var appliedV, outboxPending sql.NullInt64
		var modelV, swV sql.NullString
		var cpuTemp, fps sql.NullFloat64
		var stateUpdated sql.NullTime

		if err := rows.Scan(
			&d.ID, &d.Name, &d.Location, &d.Direction, &d.ApiKeyHash, &d.Status, &lastSeen, &d.CreatedAt, &revokedAt,
			&appliedV, &modelV, &swV, &cpuTemp, &fps, &outboxPending, &stateUpdated,
		); err != nil {
			return nil, err
		}

		if lastSeen.Valid {
			d.LastSeen = &lastSeen.Time
		}
		if revokedAt.Valid {
			d.RevokedAt = &revokedAt.Time
		}

		if appliedV.Valid {
			st := &domain.DeviceState{
				DeviceID:       d.ID,
				AppliedVersion: int(appliedV.Int64),
				ModelVersion:   modelV.String,
				SwVersion:      swV.String,
				OutboxPending:  int(outboxPending.Int64),
			}
			if cpuTemp.Valid {
				st.CPUTemp = &cpuTemp.Float64
			}
			if fps.Valid {
				st.FPS = &fps.Float64
			}
			if stateUpdated.Valid {
				st.UpdatedAt = stateUpdated.Time
			}
			d.State = st
		}

		devices = append(devices, &d)
	}
	return devices, nil
}

func (r *deviceRepo) Create(ctx context.Context, device *domain.Device) error {
	_, err := r.db.ExecContext(ctx,
		"INSERT INTO devices (id, name, location, direction, api_key_hash, status, created_at) VALUES (?, ?, ?, ?, ?, ?, ?)",
		device.ID, device.Name, device.Location, device.Direction, device.ApiKeyHash, device.Status, device.CreatedAt)
	if err != nil {
		return err
	}
	_, _ = r.db.ExecContext(ctx, "INSERT OR IGNORE INTO device_state (device_id, applied_version, updated_at) VALUES (?, 0, ?)", device.ID, device.CreatedAt)
	return nil
}

func (r *deviceRepo) UpdateStatus(ctx context.Context, id string, status string, lastSeen time.Time) error {
	_, err := r.db.ExecContext(ctx, "UPDATE devices SET status = ?, last_seen = ? WHERE id = ?", status, lastSeen, id)
	return err
}

func (r *deviceRepo) UpdateState(ctx context.Context, state *domain.DeviceState) error {
	query := `
	INSERT INTO device_state (device_id, applied_version, model_version, sw_version, cpu_temp, fps, outbox_pending, updated_at)
	VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	ON CONFLICT(device_id) DO UPDATE SET
		applied_version = excluded.applied_version,
		model_version = excluded.model_version,
		sw_version = excluded.sw_version,
		cpu_temp = excluded.cpu_temp,
		fps = excluded.fps,
		outbox_pending = excluded.outbox_pending,
		updated_at = excluded.updated_at
	`
	_, err := r.db.ExecContext(ctx, query,
		state.DeviceID, state.AppliedVersion, state.ModelVersion, state.SwVersion, state.CPUTemp, state.FPS, state.OutboxPending, state.UpdatedAt)
	return err
}

func (r *deviceRepo) Delete(ctx context.Context, id string) error {
	_, err := r.db.ExecContext(ctx, "DELETE FROM devices WHERE id = ?", id)
	return err
}
