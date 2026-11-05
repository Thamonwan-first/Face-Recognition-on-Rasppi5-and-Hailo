package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"strconv"
	"time"

	"attendance-backend/internal/entity"
	"attendance-backend/internal/ports"
)

type settingsRepo struct {
	db *sql.DB
}

func NewSettingsRepository(db *sql.DB) ports.SettingsRepository {
	return &settingsRepo{db: db}
}

func (r *settingsRepo) GetSettings(ctx context.Context) (*entity.SystemSettings, error) {
	query := `SELECT key, value, updated_at FROM system_settings`
	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	settings := &entity.SystemSettings{
		WorkStartTime:        "08:30",
		WorkEndTime:          "17:30",
		MinWorkHours:         8.0,
		DebounceSeconds:      60,
		RecognitionThreshold: 0.65,
		UpdatedAt:            time.Now(),
	}

	for rows.Next() {
		var k, v string
		var t time.Time
		if err := rows.Scan(&k, &v, &t); err != nil {
			return nil, err
		}
		if t.After(settings.UpdatedAt) {
			settings.UpdatedAt = t
		}
		switch k {
		case "work_start_time":
			settings.WorkStartTime = v
		case "work_end_time":
			settings.WorkEndTime = v
		case "min_work_hours":
			if val, err := strconv.ParseFloat(v, 64); err == nil {
				settings.MinWorkHours = val
			}
		case "debounce_seconds":
			if val, err := strconv.Atoi(v); err == nil {
				settings.DebounceSeconds = val
			}
		case "recognition_threshold":
			if val, err := strconv.ParseFloat(v, 64); err == nil {
				settings.RecognitionThreshold = val
			}
		}
	}

	return settings, nil
}

func (r *settingsRepo) UpdateSettings(ctx context.Context, s *entity.SystemSettings) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	pairs := map[string]string{
		"work_start_time":        s.WorkStartTime,
		"work_end_time":          s.WorkEndTime,
		"min_work_hours":         fmt.Sprintf("%.1f", s.MinWorkHours),
		"debounce_seconds":       strconv.Itoa(s.DebounceSeconds),
		"recognition_threshold":  fmt.Sprintf("%.2f", s.RecognitionThreshold),
	}

	query := `
	INSERT INTO system_settings (key, value, updated_at)
	VALUES ($1, $2, CURRENT_TIMESTAMP)
	ON CONFLICT (key) DO UPDATE
	SET value = EXCLUDED.value, updated_at = CURRENT_TIMESTAMP
	`

	for k, v := range pairs {
		if _, err := tx.ExecContext(ctx, query, k, v); err != nil {
			return err
		}
	}

	return tx.Commit()
}
