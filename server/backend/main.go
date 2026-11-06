package main

import (
	"context"
	"log"
	"os"
	"time"

	"attendance-backend/internal/adapters/handler/http"
	"attendance-backend/internal/adapters/repository/postgres"
	"attendance-backend/internal/pkg/broker"
	"attendance-backend/internal/pkg/ratelimit"
	"attendance-backend/internal/pkg/token"
	"attendance-backend/internal/usecase"
)

func main() {
	log.Println("==================================================")
	log.Println("🚀 Starting Face Recognition Backend (PostgreSQL)")
	log.Println("==================================================")

	databaseURL := os.Getenv("DATABASE_URL")

	jwtSecret := os.Getenv("JWT_SECRET")
	isProd := os.Getenv("ENV") == "production" || os.Getenv("GIN_MODE") == "release"
	if jwtSecret == "" {
		if isProd {
			log.Fatalf("FATAL: JWT_SECRET environment variable is required in production mode.")
		}
		log.Println("⚠️ WARNING: JWT_SECRET is not set. Using fallback development key. DO NOT USE IN PRODUCTION.")
		jwtSecret = "attendance-dev-fallback-secret-key-change-in-prod"
	}

	uploadDir := os.Getenv("UPLOAD_DIR")
	if uploadDir == "" {
		uploadDir = "data/enrollment_images"
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "8443"
	}

	// 1. Adapters: PostgreSQL Database Initialization
	pgDB, err := postgres.NewPostgresDB(databaseURL)
	if err != nil {
		log.Fatalf("Failed to initialize PostgreSQL database: %v", err)
	}
	defer pgDB.Close()
	log.Println("🐘 PostgreSQL database connected and schema ready.")

	adminRepo := postgres.NewAdminRepository(pgDB)
	deviceRepo := postgres.NewDeviceRepository(pgDB)
	userRepo := postgres.NewUserRepository(pgDB)
	attendanceRepo := postgres.NewAttendanceRepository(pgDB)
	enrollmentRepo := postgres.NewEnrollmentRepository(pgDB)
	auditRepo := postgres.NewAuditRepository(pgDB)
	settingsRepo := postgres.NewSettingsRepository(pgDB)

	// 2. Infrastructure: Security Token Maker
	tokenMaker := token.NewTokenMaker(jwtSecret)

	// 3. Core: Use Cases
	authUC := usecase.NewAuthUseCase(adminRepo, auditRepo, tokenMaker)
	deviceUC := usecase.NewDeviceUseCase(deviceRepo, auditRepo)
	userUC := usecase.NewUserUseCase(userRepo, auditRepo)
	attendanceUC := usecase.NewAttendanceUseCase(attendanceRepo, auditRepo, userRepo, settingsRepo)
	syncUC := usecase.NewSyncUseCase(deviceRepo, userRepo, attendanceUC, enrollmentRepo)
	enrollmentUC := usecase.NewEnrollmentUseCase(enrollmentRepo, userRepo, uploadDir, auditRepo, deviceRepo)
	auditUC := usecase.NewAuditUseCase(auditRepo)
	settingsUC := usecase.NewSettingsUseCase(settingsRepo, auditRepo)

	// 4. Infrastructure: Real-time Event Broker (SSE)
	eventBroker := broker.NewEventBroker()

	// 5. Adapters: HTTP Handlers
	handlers := &http.Handlers{
		Auth:       http.NewAuthHandler(authUC),
		User:       http.NewUserHandler(userUC),
		Attendance: http.NewAttendanceHandler(attendanceUC, eventBroker),
		Device:     http.NewDeviceHandler(deviceUC, eventBroker),
		Sync:       http.NewSyncHandler(syncUC, eventBroker),
		Enrollment: http.NewEnrollmentHandler(enrollmentUC, eventBroker, auditRepo, userRepo),
		Audit:      http.NewAuditHandler(auditUC),
		Realtime:   http.NewRealtimeHandler(eventBroker),
		Settings:   http.NewSettingsHandler(settingsUC, eventBroker),
	}

	// 6. Infrastructure: In-Memory Token Bucket Rate Limiter
	// 20 requests/sec with burst capacity of 40 tokens, 10 min TTL
	rateLimiter := ratelimit.NewTokenBucketLimiter(20.0, 40, 10*time.Minute)
	defer rateLimiter.Close()

	// 7. Router Setup
	router := http.SetupRouter(handlers, tokenMaker, deviceUC, rateLimiter)

	// 8. Background Watcher: Instant Device Offline Detection (runs every 2s)
	go func() {
		ticker := time.NewTicker(2 * time.Second)
		defer ticker.Stop()
		for range ticker.C {
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			res, err := pgDB.ExecContext(ctx, `
				UPDATE devices 
				SET status = 'offline' 
				WHERE status = 'online' 
				  AND last_seen IS NOT NULL 
				  AND last_seen < NOW() - INTERVAL '10 seconds'
			`)
			cancel()
			if err == nil {
				if rows, _ := res.RowsAffected(); rows > 0 {
					eventBroker.Publish(broker.Event{
						Type:    "device",
						Payload: map[string]interface{}{"status": "offline", "count": rows},
					})
				}
			}
		}
	}()

	log.Printf("Server listening on :%s", port)
	if err := router.Run(":" + port); err != nil {
		log.Fatalf("Server failed to run: %v", err)
	}
}
