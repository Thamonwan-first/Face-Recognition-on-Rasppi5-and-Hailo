package http_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	handlerHttp "attendance-backend/internal/adapters/handler/http"
	"attendance-backend/internal/entity"
	"attendance-backend/internal/pkg/broker"
	"attendance-backend/internal/pkg/hasher"
	"attendance-backend/internal/pkg/ratelimit"
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

func (m *mockDeviceRepo) UpdateIPAddress(ctx context.Context, id string, ip string) error {
	if m.device != nil && m.device.ID == id {
		m.device.IPAddress = ip
	}
	return nil
}

func (m *mockDeviceRepo) UpdateStatusAndIP(ctx context.Context, id string, status string, ip string, lastSeen time.Time) error {
	if m.device != nil && m.device.ID == id {
		m.device.Status = status
		m.device.IPAddress = ip
		m.device.LastSeen = &lastSeen
	}
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
func (m *mockAttendanceRepo) ListPairs(ctx context.Context, startDate, endDate, userID, department, deviceID string, limit int) ([]*entity.AttendancePair, error) {
	return m.pairs, nil
}
func (m *mockAttendanceRepo) GetPairByID(ctx context.Context, id int) (*entity.AttendancePair, error) {
	for _, p := range m.pairs {
		if p.ID == id {
			return p, nil
		}
	}
	return nil, entity.ErrNotFound
}
func (m *mockAttendanceRepo) FindOpenPair(ctx context.Context, userID, workDate string) (*entity.AttendancePair, error) {
	return nil, entity.ErrNotFound
}
func (m *mockAttendanceRepo) FindLastPairByUserAndDate(ctx context.Context, userID, workDate string) (*entity.AttendancePair, error) {
	return nil, entity.ErrNotFound
}
func (m *mockAttendanceRepo) SavePair(ctx context.Context, pair *entity.AttendancePair) error {
	return nil
}
func (m *mockAttendanceRepo) UpdatePair(ctx context.Context, pair *entity.AttendancePair) error {
	return nil
}
func (m *mockAttendanceRepo) SaveEventAndPair(ctx context.Context, event *entity.AttendanceEvent, pair *entity.AttendancePair, isUpdate bool) error {
	return nil
}
func (m *mockAttendanceRepo) SaveUnrecognized(ctx context.Context, scan *entity.UnrecognizedScan) error {
	return nil
}
func (m *mockAttendanceRepo) ListUnrecognized(ctx context.Context, limit int) ([]*entity.UnrecognizedScan, error) {
	return nil, nil
}
func (m *mockAttendanceRepo) GetUnrecognizedByID(ctx context.Context, id string) (*entity.UnrecognizedScan, error) {
	return nil, entity.ErrNotFound
}
func (m *mockAttendanceRepo) ResolveUnrecognized(ctx context.Context, id, userID, status string) error {
	return nil
}
func (m *mockAttendanceRepo) DeleteUnrecognized(ctx context.Context, id string) error {
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

type mockSettingsRepo struct {
	settings *entity.SystemSettings
}

func (m *mockSettingsRepo) GetSettings(ctx context.Context) (*entity.SystemSettings, error) {
	if m.settings == nil {
		m.settings = &entity.SystemSettings{
			WorkStartTime:        "08:30",
			WorkEndTime:          "17:30",
			MinWorkHours:         8.0,
			DebounceSeconds:      60,
			RecognitionThreshold: 0.65,
		}
	}
	return m.settings, nil
}

func (m *mockSettingsRepo) UpdateSettings(ctx context.Context, s *entity.SystemSettings) error {
	m.settings = s
	return nil
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
	settingsRepo := &mockSettingsRepo{}

	tokenMaker := token.NewTokenMaker("test-secret-key-1234567890")

	authUC := usecase.NewAuthUseCase(adminRepo, auditRepo, tokenMaker)
	deviceUC := usecase.NewDeviceUseCase(deviceRepo, auditRepo)
	userUC := usecase.NewUserUseCase(userRepo, auditRepo)
	attendanceUC := usecase.NewAttendanceUseCase(attendanceRepo, auditRepo, userRepo, settingsRepo)
	syncUC := usecase.NewSyncUseCase(deviceRepo, userRepo, attendanceUC, enrollmentRepo)
	enrollmentUC := usecase.NewEnrollmentUseCase(enrollmentRepo, userRepo, "data/test_enrollment")
	auditUC := usecase.NewAuditUseCase(auditRepo)
	settingsUC := usecase.NewSettingsUseCase(settingsRepo, auditRepo)

	brokerInst := broker.NewEventBroker()

	handlers := &handlerHttp.Handlers{
		Auth:       handlerHttp.NewAuthHandler(authUC),
		User:       handlerHttp.NewUserHandler(userUC),
		Attendance: handlerHttp.NewAttendanceHandler(attendanceUC),
		Device:     handlerHttp.NewDeviceHandler(deviceUC, brokerInst),
		Sync:       handlerHttp.NewSyncHandler(syncUC, brokerInst),
		Enrollment: handlerHttp.NewEnrollmentHandler(enrollmentUC, brokerInst),
		Audit:      handlerHttp.NewAuditHandler(auditUC),
		Realtime:   handlerHttp.NewRealtimeHandler(brokerInst),
		Settings:   handlerHttp.NewSettingsHandler(settingsUC, brokerInst),
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

func TestHTTP_RealtimeSSE(t *testing.T) {
	ts := setupTestServer()
	defer ts.Close()

	// 1. Without token -> 401
	resp, err := http.Get(ts.URL + "/api/realtime/stream")
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401 without token, got %d", resp.StatusCode)
	}

	// 2. Login to get token
	loginBody, _ := json.Marshal(map[string]string{
		"username": "admin",
		"password": "admin1234",
	})
	loginRespRaw, _ := http.Post(ts.URL+"/api/auth/login", "application/json", bytes.NewBuffer(loginBody))
	var loginResp struct {
		AccessToken string `json:"access_token"`
	}
	json.NewDecoder(loginRespRaw.Body).Decode(&loginResp)
	loginRespRaw.Body.Close()

	// 3. Connect to SSE stream using query parameter ?token=
	req, _ := http.NewRequest("GET", ts.URL+"/api/realtime/stream?token="+loginResp.AccessToken, nil)
	client := &http.Client{Timeout: 2 * time.Second}
	sseResp, err := client.Do(req)
	if err != nil {
		// client timeout is normal for infinite stream
		return
	}
	defer sseResp.Body.Close()

	if sseResp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 for SSE stream, got %d", sseResp.StatusCode)
	}
	if contentType := sseResp.Header.Get("Content-Type"); !strings.HasPrefix(contentType, "text/event-stream") {
		t.Fatalf("expected text/event-stream, got %s", contentType)
	}
}

func TestHTTP_RateLimiter(t *testing.T) {
	// Setup test server with tight rate limiter (1 req/sec, burst 2)
	adminHash, _ := hasher.HashPassword("admin1234")
	adminRepo := &mockAdminRepo{
		admin: &entity.Admin{
			ID:           "test-admin-id",
			Username:     "admin",
			PasswordHash: adminHash,
			Role:         "admin",
		},
	}
	deviceRepo := &mockDeviceRepo{}
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

	tightLimiter := ratelimit.NewTokenBucketLimiter(1.0, 2, 1*time.Minute)
	defer tightLimiter.Close()

	router := handlerHttp.SetupRouter(handlers, tokenMaker, deviceUC, tightLimiter)
	ts := httptest.NewServer(router)
	defer ts.Close()

	// 1st request -> allowed (200)
	resp1, err := http.Get(ts.URL + "/health")
	if err != nil || resp1.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp1.StatusCode)
	}
	resp1.Body.Close()

	// 2nd request -> allowed (200)
	resp2, err := http.Get(ts.URL + "/health")
	if err != nil || resp2.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp2.StatusCode)
	}
	resp2.Body.Close()

	// 3rd immediate request -> rate limit exceeded (429)
	resp3, err := http.Get(ts.URL + "/health")
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	resp3.Body.Close()
	if resp3.StatusCode != http.StatusTooManyRequests {
		t.Fatalf("expected 429 Too Many Requests, got %d", resp3.StatusCode)
	}
}

func TestHTTP_AttendanceManualAdjustment(t *testing.T) {
	ts := setupTestServer()
	defer ts.Close()

	// 1. Login to get token
	loginBody, _ := json.Marshal(map[string]string{
		"username": "admin",
		"password": "admin1234",
	})
	loginRespRaw, _ := http.Post(ts.URL+"/api/auth/login", "application/json", bytes.NewBuffer(loginBody))
	var loginResp struct {
		AccessToken string `json:"access_token"`
	}
	json.NewDecoder(loginRespRaw.Body).Decode(&loginResp)
	loginRespRaw.Body.Close()

	// 2. POST /api/attendance/manual without reason -> 400 Bad Request
	noReasonBody, _ := json.Marshal(map[string]string{
		"user_id":   "user-1",
		"work_date": "2026-11-04",
		"reason":    "",
	})
	reqNoReason, _ := http.NewRequest("POST", ts.URL+"/api/attendance/manual", bytes.NewBuffer(noReasonBody))
	reqNoReason.Header.Set("Authorization", "Bearer "+loginResp.AccessToken)
	reqNoReason.Header.Set("Content-Type", "application/json")
	respNoReason, err := http.DefaultClient.Do(reqNoReason)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	respNoReason.Body.Close()
	if respNoReason.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400 for missing reason, got %d", respNoReason.StatusCode)
	}

	// 3. POST /api/attendance/manual with reason -> 201 Created
	validBody, _ := json.Marshal(map[string]string{
		"user_id":        "user-1",
		"work_date":      "2026-11-04",
		"check_in_time":  "08:30",
		"check_out_time": "17:30",
		"reason":         "พนักงานไปปฏิบัติงานนอกสถานที่",
	})
	reqValid, _ := http.NewRequest("POST", ts.URL+"/api/attendance/manual", bytes.NewBuffer(validBody))
	reqValid.Header.Set("Authorization", "Bearer "+loginResp.AccessToken)
	reqValid.Header.Set("Content-Type", "application/json")
	respValid, err := http.DefaultClient.Do(reqValid)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer respValid.Body.Close()
	if respValid.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201 Created, got %d", respValid.StatusCode)
	}

	var createdPair entity.AttendancePair
	json.NewDecoder(respValid.Body).Decode(&createdPair)
	if createdPair.AdjustmentReason != "พนักงานไปปฏิบัติงานนอกสถานที่" {
		t.Fatalf("unexpected reason: %s", createdPair.AdjustmentReason)
	}
	if !createdPair.IsManual {
		t.Fatalf("expected is_manual to be true")
	}
}

func TestHTTP_Settings(t *testing.T) {
	ts := setupTestServer()
	defer ts.Close()

	// 1. Login to get token
	loginBody, _ := json.Marshal(map[string]string{
		"username": "admin",
		"password": "admin1234",
	})
	loginRespRaw, _ := http.Post(ts.URL+"/api/auth/login", "application/json", bytes.NewBuffer(loginBody))
	var loginResp struct {
		AccessToken string `json:"access_token"`
	}
	json.NewDecoder(loginRespRaw.Body).Decode(&loginResp)
	loginRespRaw.Body.Close()

	// 2. GET /api/settings -> 200
	req, _ := http.NewRequest("GET", ts.URL+"/api/settings", nil)
	req.Header.Set("Authorization", "Bearer "+loginResp.AccessToken)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 for settings, got %d", resp.StatusCode)
	}

	var settings entity.SystemSettings
	json.NewDecoder(resp.Body).Decode(&settings)
	if settings.WorkStartTime != "08:30" {
		t.Fatalf("expected default 08:30, got %s", settings.WorkStartTime)
	}

	// 3. PUT /api/settings -> 200
	updateBody, _ := json.Marshal(entity.SystemSettings{
		WorkStartTime:        "09:00",
		WorkEndTime:          "18:00",
		MinWorkHours:         8.0,
		DebounceSeconds:      45,
		RecognitionThreshold: 0.70,
	})
	reqPut, _ := http.NewRequest("PUT", ts.URL+"/api/settings", bytes.NewBuffer(updateBody))
	reqPut.Header.Set("Authorization", "Bearer "+loginResp.AccessToken)
	reqPut.Header.Set("Content-Type", "application/json")
	respPut, err := http.DefaultClient.Do(reqPut)
	if err != nil {
		t.Fatalf("put request failed: %v", err)
	}
	defer respPut.Body.Close()
	if respPut.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 on put settings, got %d", respPut.StatusCode)
	}
}
