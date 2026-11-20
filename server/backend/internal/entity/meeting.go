package entity

import "time"

type Meeting struct {
	ID                   string     `json:"id"`
	Title                string     `json:"title"`
	Description          string     `json:"description"`
	Location             string     `json:"location"`
	DeviceID             *string    `json:"device_id,omitempty"`
	DeviceName           string     `json:"device_name,omitempty"`
	MeetingDate          string     `json:"meeting_date"` // YYYY-MM-DD
	StartTime            string     `json:"start_time"`   // HH:MM (e.g. "13:30")
	EndTime              string     `json:"end_time"`     // HH:MM (e.g. "15:00")
	LateThresholdMinutes int        `json:"late_threshold_minutes"`
	Status               string     `json:"status"` // "scheduled", "ongoing", "completed", "cancelled"
	CreatedBy            string     `json:"created_by,omitempty"`
	AttendeeCount        int        `json:"attendee_count"`
	CreatedAt            time.Time  `json:"created_at"`
	UpdatedAt            time.Time  `json:"updated_at"`
}

type MeetingAttendee struct {
	ID          int       `json:"id"`
	MeetingID   string    `json:"meeting_id"`
	UserID      string    `json:"user_id"`
	UserName    string    `json:"user_name,omitempty"`
	EmployeeID  string    `json:"employee_id,omitempty"`
	Department  string    `json:"department,omitempty"`
	EventID     string    `json:"event_id,omitempty"`
	ScannedAt   time.Time `json:"scanned_at"`
	SnapshotURL string    `json:"snapshot_url"`
	Status      string    `json:"status"` // "on_time", "late"
	MinutesLate int       `json:"minutes_late"`
	Notes       string    `json:"notes,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
}
