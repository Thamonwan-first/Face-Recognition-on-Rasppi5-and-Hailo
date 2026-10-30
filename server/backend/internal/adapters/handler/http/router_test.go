package http_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	handlerHttp "attendance-backend/internal/adapters/handler/http"
	"attendance-backend/internal/entity"
	"attendance-backend/internal/pkg/hasher"
	"attendance-backend/internal/pkg/token"
	"attendance-backend/internal/usecase"
)

// In-Memory Mock Repositories for fast, zero-dependency HTTP testing
type mockAdminRepo struct {
	admin *entity.Admin
}

func (m *mockAdminRepo) GetByUsername(ctx context.Context, username string) (*entity.Admin, error) {
	if m.admin != nil && m.admin.Username == username {
		return m.admin, nil
	}
	return nil, entity.ErrNotFound
}

func (m *mockAdminRepo) GetByID(ctx context.Context, id string) (*entity.Admin, error) {
	if m.admin != nil && m.admin.ID == id {
		return m.admin, nil
	}
	return nil, entity.ErrNotFound
}

func (m *mockAdminRepo) List(ctx context.Context) ([]*entity.Admin, error) {
	if m.admin != nil {
		return []*entity.Admin{m.admin}, nil
	}
	return []*entity.Admin{}, nil
}

func (m *mockAdminRepo) Create(ctx context.Context, admin *entity.Admin) error {
	m.admin = admin
	return nil
}

func (m *mockAdminRepo) Update(ctx context.Context, admin *entity.Admin) error {
	m.admin = admin
	return nil
}

func (m *mockAdminRepo) UpdatePassword(ctx context.Context, id string, passwordHash string) error {
	if m.admin != nil {
		m.admin.PasswordHash = passwordHash
	}
	return nil
}

func (m *mockAdminRepo) UpdateLastLogin(ctx context.Context, id string, lastLogin time.Time) error {
	return nil
}

func (m *mockAdminRepo) Delete(ctx context.Context, id string) error {
	m.admin = nil
	return nil
}

type mockDeviceRepo struct {
	device *entity.Device
}

func (m *mockDeviceRepo) GetByID(ctx context.Context, id string) (*entity.Device, error) {
	if m.device != nil && m.device.ID == id {
		return m.device, nil
	}
	return nil, entity.ErrNotFound
}

func (m *mockDeviceRepo) GetByApiKeyHash(ctx context.Context, hash string) (*entity.Device, error) {
	if m.device != nil && m.device.ApiKeyHash == hash {
		return m.device, nil
	}
	return nil, entity.ErrNotFound
}

func (m *mockDeviceRepo) List(ctx context.Context) ([]*entity.Device, error) {
	if m.device != nil {
		return []*entity.Device{m.device}, nil
	}
	return []*entity.Device{}, nil
}

func (m *mockDeviceRepo) Create(ctx context.Context, device *entity.Device) error {
	m.device = device
	return nil
}

func (m *mockDeviceRepo) UpdateStatus(ctx context.Context, id string, status string, lastSeen time.Time) error {
	return nil
}

func (m *mockDeviceRepo) UpdateState(ctx context.Context, state *entity.DeviceState) error {
	return nil
}

func (m *mockDeviceRepo) Delete(ctx context.Context, id string) error {
	m.device = nil
	return nil
}

type mockUserRepo struct {
	users []*entity.User
}

func (m *mockUserRepo) GetByID(ctx context.Context, id string) (*entity.User, error) {
	return nil, entity.ErrNotFound
}
func (m *mockUserRepo) GetByEmployeeID(ctx context.Context, employeeID string) (*entity.User, error) {
	return nil, entity.ErrNotFound
}
func (m *mockUserRepo) List(ctx context.Context, unenrolledOnly bool) ([]*entity.User, error) {
	return m.users, nil
}
func (m *mockUserRepo) Create(ctx context.Context, user *entity.User) error {
	m.users = append(m.users, user)
	return nil
}
func (m *mockUserRepo) Update(ctx context.Context, user *entity.User) error { return nil }
func (m *mockUserRepo) Delete(ctx context.Context, id string) error          { return nil }
func (m *mockUserRepo) GetLatestVersion(ctx context.Context) (int, error)    { return 1, nil }
func (m *mockUserRepo) ListUpdatedSince(ctx context.Context, sinceVersion int) ([]*entity.User, error) {
	return m.users, nil
}
func (m *mockUserRepo) SaveEmbedding(ctx context.Context, embedding *entity.UserEmbedding) error {
	return nil
}
func (m *mockUserRepo) GetEmbedding(ctx context.Context, userID string) (*entity.UserEmbedding, error) {
	return nil, entity.ErrNotFound
}
func (m *mockUserRepo) DeleteEmbedding(ctx context.Context, userID string) error { return nil }

type mockAttendanceRepo struct {
	pairs []*entity.AttendancePair
}

func (m *mockAttendanceRepo) SaveEvent(ctx context.Context, event *entity.AttendanceEvent) error {
	return nil
}
func (m *mockAttendanceRepo) GetEventByID(ctx context.Context, eventID string) (*entity.AttendanceEvent, error) {
	return nil, entity.ErrNotFound
}
func (m *mockAttendanceRepo) ListEvents(ctx context.Context, limit int) ([]*entity.AttendanceEvent, error) {
	return nil, nil
}
func (m *mockAttendanceRepo) ListPairs(ctx context.Context, startDate, endDate, userID, department string, limit int) ([]*entity.AttendancePair, error) {
	return m.pairs, nil
}
func (m *mockAttendanceRepo) FindOpenPair(ctx context.Context, userID, workDate string) (*entity.AttendancePair, error) {
	return nil, entity.ErrNotFound
}
func (m *mockAttendanceRepo) SavePair(ctx context.Context, pair *entity.AttendancePair) error {
	return nil
}
func (m *mockAttendanceRepo) UpdatePair(ctx context.Context, pair *entity.AttendancePair) error {
	return nil
}

type mockEnrollmentRepo struct{}

func (m *mockEnrollmentRepo) CreateJob(ctx context.Context, job *entity.EnrollmentJob) error {
	return nil
}
func (m *mockEnrollmentRepo) ClaimPendingJob(ctx context.Context, deviceID string) (*entity.EnrollmentJob, error) {
	return nil, nil
}
func (m *mockEnrollmentRepo) GetPendingJobs(ctx context.Context) ([]*entity.EnrollmentJob, error) {
	return nil, nil
}
func (m *mockEnrollmentRepo) GetJobByID(ctx context.Context, id string) (*entity.EnrollmentJob, error) {
	return nil, entity.ErrNotFound
}
func (m *mockEnrollmentRepo) UpdateJobStatus(ctx context.Context, id, status string) error {
	return nil
}

type mockAuditRepo struct{}

func (m *mockAuditRepo) Log(ctx context.Context, log *entity.AuditLog) error { return nil }
func (m *mockAuditRepo) List(ctx context.Context, limit int) ([]*entity.AuditLog, error) {
	return nil, nil
}

func setupTestServer() *httptest.Server {
	adminHash, _ := hasher.HashPassword("admin1234")
	adminRepo := &mockAdminRepo{
		admin: &entity.Admin{
			ID:           "test-admin-id",
			Username:     "admin",
			PasswordHash: adminHash,
			Role:         "admin",
		},
	}

	deviceKey := "changeme-generate-with-openssl-rand-hex-32"
	deviceHash := hasher.HashAPIKey(deviceKey)
	deviceRepo := &mockDeviceRepo{
		device: &entity.Device{
			ID:         "kiosk-01",
			Name:       "kiosk-01",
			ApiKeyHash: deviceHash,
			Status:     "online",
		},
	}

	userRepo := &mockUserRepo{}
	attendanceRepo := &mockAttendanceRepo{}
	enrollmentRepo := &mockEnrollmentRepo{}
	auditRepo := &mockAuditRepo{}

	tokenMaker := token.NewTokenMaker("test-secret-key-1234567890")

	authUC := usecase.NewAuthUseCase(adminRepo, auditRepo, tokenMaker)
	deviceUC := usecase.NewDeviceUseCase(deviceRepo, auditRepo)
	userUC := usecase.NewUserUseCase(userRepo, auditRepo)
	attendanceUC := usecase.NewAttendanceUseCase(attendanceRepo)
	syncUC := usecase.NewSyncUseCase(deviceRepo, userRepo, attendanceUC, enrollmentRepo)
	enrollmentUC := usecase.NewEnrollmentUseCase(enrollmentRepo, userRepo, "data/test_enrollment")
	auditUC := usecase.NewAuditUseCase(auditRepo)

	handlers := &handlerHttp.Handlers{
		Auth:       handlerHttp.NewAuthHandler(authUC),
		User:       handlerHttp.NewUserHandler(userUC),
		Attendance: handlerHttp.NewAttendanceHandler(attendanceUC),
		Device:     handlerHttp.NewDeviceHandler(deviceUC),
		Sync:       handlerHttp.NewSyncHandler(syncUC),
		Enrollment: handlerHttp.NewEnrollmentHandler(enrollmentUC),
		Audit:      handlerHttp.NewAuditHandler(auditUC),
	}

	router := handlerHttp.SetupRouter(handlers, tokenMaker, deviceUC)
	return httptest.NewServer(router)
}

func TestHTTP_HealthCheck(t *testing.T) {
	ts := setupTestServer()
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/health")
	if err != nil {
		t.Fatalf("GET /health failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
}

func TestHTTP_AuthLogin_And_ProtectedRoutes(t *testing.T) {
	ts := setupTestServer()
	defer ts.Close()

	// 1. Invalid login
	body, _ := json.Marshal(map[string]string{"username": "admin", "password": "wrongpassword"})
	resp, err := http.Post(ts.URL+"/auth/login", "application/json", bytes.NewBuffer(body))
	if err != nil {
		t.Fatalf("login request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401 for wrong password, got %d", resp.StatusCode)
	}

	// 2. Valid login
	body, _ = json.Marshal(map[string]string{"username": "admin", "password": "admin1234"})
	resp, err = http.Post(ts.URL+"/auth/login", "application/json", bytes.NewBuffer(body))
	if err != nil {
		t.Fatalf("login request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 for valid login, got %d", resp.StatusCode)
	}

	var loginResp struct {
		AccessToken string `json:"access_token"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&loginResp)
	if loginResp.AccessToken == "" {
		t.Fatalf("expected non-empty access token")
	}

	// 3. Access protected route without token -> 401
	respNoAuth, err := http.Get(ts.URL + "/api/attendance/pairs")
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer respNoAuth.Body.Close()
	if respNoAuth.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401 without token, got %d", respNoAuth.StatusCode)
	}

	// 4. Access protected route with token -> 200
	req, _ := http.NewRequest("GET", ts.URL+"/api/attendance/pairs", nil)
	req.Header.Set("Authorization", "Bearer "+loginResp.AccessToken)
	respAuth, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("authenticated request failed: %v", err)
	}
	defer respAuth.Body.Close()
	if respAuth.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 with valid token, got %d", respAuth.StatusCode)
	}
}

func TestHTTP_DeviceAuth(t *testing.T) {
	ts := setupTestServer()
	defer ts.Close()

	// 1. Without API key -> 401
	resp, err := http.Get(ts.URL + "/sync/users")
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401 without API key, got %d", resp.StatusCode)
	}

	// 2. With valid seeded API key -> 200
	req, _ := http.NewRequest("GET", ts.URL+"/sync/users", nil)
	req.Header.Set("X-API-Key", "changeme-generate-with-openssl-rand-hex-32")
	respAuth, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer respAuth.Body.Close()
	if respAuth.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 with valid device API key, got %d", respAuth.StatusCode)
	}
}
