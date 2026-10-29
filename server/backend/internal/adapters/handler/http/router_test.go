package http_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	handlerHttp "attendance-backend/internal/adapters/handler/http"
	"attendance-backend/internal/adapters/repository/sqlite"
	"attendance-backend/internal/pkg/token"
	"attendance-backend/internal/usecase"
)

func setupTestServer(t *testing.T) (*httptest.Server, *sqlite.NewTestDBHelper) {
	tempDB := "test_server.db"
	_ = os.Remove(tempDB)

	db, err := sqlite.NewSQLiteDB(tempDB)
	if err != nil {
		t.Fatalf("failed to open test sqlite db: %v", err)
	}

	adminRepo := sqlite.NewAdminRepository(db)
	deviceRepo := sqlite.NewDeviceRepository(db)
	userRepo := sqlite.NewUserRepository(db)
	attendanceRepo := sqlite.NewAttendanceRepository(db)
	enrollmentRepo := sqlite.NewEnrollmentRepository(db)
	auditRepo := sqlite.NewAuditRepository(db)

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
	ts := httptest.NewServer(router)

	cleanup := func() {
		ts.Close()
		db.Close()
		_ = os.Remove(tempDB)
		_ = os.RemoveAll("data/test_enrollment")
	}

	return ts, &sqlite.NewTestDBHelper{Cleanup: cleanup}
}

func TestHTTP_HealthCheck(t *testing.T) {
	ts, h := setupTestServer(t)
	defer h.Cleanup()

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
	ts, h := setupTestServer(t)
	defer h.Cleanup()

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

	// 2. Valid login (seeded default admin)
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
	ts, h := setupTestServer(t)
	defer h.Cleanup()

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
