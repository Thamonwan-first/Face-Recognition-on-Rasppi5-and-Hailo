package entity

import "time"

type User struct {
	ID           string     `json:"id"`
	EmployeeID   string     `json:"employee_id"`
	Title        string     `json:"title"`
	FirstName    string     `json:"first_name"`
	LastName     string     `json:"last_name"`
	Name         string     `json:"name"`
	Department   string     `json:"department"`
	Status       string     `json:"status"`
	Version      int        `json:"version"`
	DeletedAt    *time.Time `json:"deleted_at"`
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`
	HasEmbedding bool       `json:"has_embedding"`
	RawEmbedding []byte     `json:"-"`
}

type UserEmbedding struct {
	ID           string    `json:"id"`
	UserID       string    `json:"user_id"`
	Embedding    []byte    `json:"embedding"`
	ModelVersion string    `json:"model_version"`
	Source       string    `json:"source"`
	DeviceID     string    `json:"device_id"`
	QualityScore float64   `json:"quality_score"`
	CreatedAt    time.Time `json:"created_at"`
}
