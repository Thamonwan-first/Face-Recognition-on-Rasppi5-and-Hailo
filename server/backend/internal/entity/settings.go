package entity

import "time"

// SystemSettings holds enterprise system-wide configuration
type SystemSettings struct {
	WorkStartTime        string    `json:"work_start_time"`        // Format: "HH:MM", e.g. "08:30"
	WorkEndTime          string    `json:"work_end_time"`          // Format: "HH:MM", e.g. "17:30"
	MinWorkHours         float64   `json:"min_work_hours"`         // Minimum hours for full-day, e.g. 8.0
	DebounceSeconds      int       `json:"debounce_seconds"`       // Kiosk re-scan debounce period, e.g. 60
	RecognitionThreshold float64   `json:"recognition_threshold"`  // Face cosine similarity threshold, e.g. 0.65
	UpdatedAt            time.Time `json:"updated_at"`
}
