package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"time"

	"attendance-backend/internal/entity"
	"attendance-backend/internal/ports"
)

type adminRepo struct {
	db *sql.DB
}

func NewAdminRepository(db *sql.DB) ports.AdminRepository {
	return &adminRepo{db: db}
}

func scanAdmin(row rowScanner) (*entity.Admin, error) {
	var a entity.Admin
	var lastLogin sql.NullTime
	var permsStr sql.NullString
	if err := row.Scan(&a.ID, &a.Username, &a.PasswordHash, &a.Role, &permsStr, &a.CreatedAt, &lastLogin); err != nil {
		if err == sql.ErrNoRows {
			return nil, entity.ErrNotFound
		}
		return nil, err
	}
	if lastLogin.Valid {
		a.LastLogin = &lastLogin.Time
	}
	if permsStr.Valid && permsStr.String != "" {
		_ = json.Unmarshal([]byte(permsStr.String), &a.Permissions)
	}
	if a.Permissions == nil {
		a.Permissions = []string{}
	}
	return &a, nil
}

func (r *adminRepo) GetByUsername(ctx context.Context, username string) (*entity.Admin, error) {
	row := r.db.QueryRowContext(ctx, "SELECT id, username, password_hash, role, permissions, created_at, last_login FROM admins WHERE username = $1", username)
	return scanAdmin(row)
}

func (r *adminRepo) GetByID(ctx context.Context, id string) (*entity.Admin, error) {
	row := r.db.QueryRowContext(ctx, "SELECT id, username, password_hash, role, permissions, created_at, last_login FROM admins WHERE id = $1", id)
	return scanAdmin(row)
}

func (r *adminRepo) List(ctx context.Context) ([]*entity.Admin, error) {
	rows, err := r.db.QueryContext(ctx, "SELECT id, username, password_hash, role, permissions, created_at, last_login FROM admins ORDER BY created_at ASC")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var admins []*entity.Admin
	for rows.Next() {
		a, err := scanAdmin(rows)
		if err != nil {
			return nil, err
		}
		admins = append(admins, a)
	}
	return admins, nil
}

func (r *adminRepo) Create(ctx context.Context, admin *entity.Admin) error {
	permsBytes, _ := json.Marshal(admin.Permissions)
	_, err := r.db.ExecContext(ctx, "INSERT INTO admins (id, username, password_hash, role, permissions, created_at) VALUES ($1, $2, $3, $4, $5, $6)",
		admin.ID, admin.Username, admin.PasswordHash, admin.Role, string(permsBytes), admin.CreatedAt)
	return err
}

func (r *adminRepo) Update(ctx context.Context, admin *entity.Admin) error {
	permsBytes, _ := json.Marshal(admin.Permissions)
	_, err := r.db.ExecContext(ctx, "UPDATE admins SET role = $1, permissions = $2 WHERE id = $3",
		admin.Role, string(permsBytes), admin.ID)
	return err
}

func (r *adminRepo) UpdatePassword(ctx context.Context, id string, passwordHash string) error {
	_, err := r.db.ExecContext(ctx, "UPDATE admins SET password_hash = $1 WHERE id = $2", passwordHash, id)
	return err
}

func (r *adminRepo) UpdateLastLogin(ctx context.Context, id string, lastLogin time.Time) error {
	_, err := r.db.ExecContext(ctx, "UPDATE admins SET last_login = $1 WHERE id = $2", lastLogin, id)
	return err
}

func (r *adminRepo) Delete(ctx context.Context, id string) error {
	_, err := r.db.ExecContext(ctx, "DELETE FROM admins WHERE id = $1", id)
	return err
}

// ---------------- Device Repo ----------------

type deviceRepo struct {
	db *sql.DB
}

func NewDeviceRepository(db *sql.DB) ports.DeviceRepository {
	return &deviceRepo{db: db}
}

func (r *deviceRepo) GetByID(ctx context.Context, id string) (*entity.Device, error) {
	row := r.db.QueryRowContext(ctx, "SELECT id, name, location, direction, api_key_hash, status, last_seen, created_at, revoked_at FROM devices WHERE id = $1", id)
	return scanDeviceRow(row)
}

func (r *deviceRepo) GetByApiKeyHash(ctx context.Context, hash string) (*entity.Device, error) {
	row := r.db.QueryRowContext(ctx, "SELECT id, name, location, direction, api_key_hash, status, last_seen, created_at, revoked_at FROM devices WHERE api_key_hash = $1 AND status != 'revoked'", hash)
	return scanDeviceRow(row)
}

func scanDeviceRow(row *sql.Row) (*entity.Device, error) {
	var d entity.Device
	var lastSeen, revokedAt sql.NullTime
	if err := row.Scan(&d.ID, &d.Name, &d.Location, &d.Direction, &d.ApiKeyHash, &d.Status, &lastSeen, &d.CreatedAt, &revokedAt); err != nil {
		if err == sql.ErrNoRows {
			return nil, entity.ErrNotFound
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

func (r *deviceRepo) List(ctx context.Context) ([]*entity.Device, error) {
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

	var devices []*entity.Device
	for rows.Next() {
		var d entity.Device
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
			st := &entity.DeviceState{
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

func (r *deviceRepo) Create(ctx context.Context, device *entity.Device) error {
	query := `
	INSERT INTO devices (id, name, location, direction, api_key_hash, status, created_at)
	VALUES ($1, $2, $3, $4, $5, $6, $7)
	`
	_, err := r.db.ExecContext(ctx, query,
		device.ID, device.Name, device.Location, device.Direction, device.ApiKeyHash, device.Status, device.CreatedAt)
	if err != nil {
		return err
	}
	_, _ = r.db.ExecContext(ctx, "INSERT INTO device_state (device_id, applied_version, updated_at) VALUES ($1, 0, $2) ON CONFLICT(device_id) DO NOTHING", device.ID, device.CreatedAt)
	return nil
}

func (r *deviceRepo) UpdateStatus(ctx context.Context, id string, status string, lastSeen time.Time) error {
	_, err := r.db.ExecContext(ctx, "UPDATE devices SET status = $1, last_seen = $2 WHERE id = $3", status, lastSeen, id)
	return err
}

func (r *deviceRepo) UpdateState(ctx context.Context, state *entity.DeviceState) error {
	query := `
	INSERT INTO device_state (device_id, applied_version, model_version, sw_version, cpu_temp, fps, outbox_pending, updated_at)
	VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	ON CONFLICT(device_id) DO UPDATE SET
		applied_version = EXCLUDED.applied_version,
		model_version = EXCLUDED.model_version,
		sw_version = EXCLUDED.sw_version,
		cpu_temp = EXCLUDED.cpu_temp,
		fps = EXCLUDED.fps,
		outbox_pending = EXCLUDED.outbox_pending,
		updated_at = EXCLUDED.updated_at
	`
	_, err := r.db.ExecContext(ctx, query,
		state.DeviceID, state.AppliedVersion, state.ModelVersion, state.SwVersion, state.CPUTemp, state.FPS, state.OutboxPending, state.UpdatedAt)
	return err
}

func (r *deviceRepo) Delete(ctx context.Context, id string) error {
	_, err := r.db.ExecContext(ctx, "DELETE FROM devices WHERE id = $1", id)
	return err
}
