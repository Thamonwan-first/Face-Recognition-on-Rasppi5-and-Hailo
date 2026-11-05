package entity

import "time"

type EnrollmentJob struct {
	ID               string     `json:"id"`
	UserID           string     `json:"user_id"`
	ImagePath        string     `json:"image_path"`
	Status           string     `json:"status"`
	AssignedDeviceID string     `json:"assigned_device_id"`
	CreatedAt        time.Time  `json:"created_at"`
	ProcessedAt      *time.Time `json:"processed_at"`
	ExpiresAt        *time.Time `json:"expires_at"`
	ImageB64         string     `json:"image_b64,omitempty"`
	ImagesB64        []string   `json:"images_b64,omitempty"`
}

