package main

import (
	"log"
	"os"

	"attendance-backend/internal/adapters/handler/http"
	"attendance-backend/internal/adapters/repository/postgres"
	"attendance-backend/internal/adapters/repository/sqlite"
	"attendance-backend/internal/core/ports"
	"attendance-backend/internal/pkg/token"
	"attendance-backend/internal/usecase"
)

func main() {
	log.Println("==================================================")
	log.Println("🚀 Starting Face Recognition Backend (Go Clean Architecture)")
	log.Println("==================================================")

	dbDialect := os.Getenv("DB_DIALECT")
	databaseURL := os.Getenv("DATABASE_URL")
	dbPath := os.Getenv("DATABASE_PATH")
	if dbPath == "" {
		dbPath = "data/server.db"
	}

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

	// 1. Adapters: Database Initialization (Supports PostgreSQL & SQLite)
	var (
		adminRepo      ports.AdminRepository
		deviceRepo     ports.DeviceRepository
		userRepo       ports.UserRepository
		attendanceRepo ports.AttendanceRepository
		enrollmentRepo ports.EnrollmentRepository
		auditRepo      ports.AuditRepository
	)

	if dbDialect == "postgres" || databaseURL != "" {
		pgDB, err := postgres.NewPostgresDB(databaseURL)
		if err != nil {
			log.Fatalf("Failed to initialize PostgreSQL database: %v", err)
		}
		defer pgDB.Close()
		log.Println("🐘 PostgreSQL database connected and schema ready.")

		adminRepo = postgres.NewAdminRepository(pgDB)
		deviceRepo = postgres.NewDeviceRepository(pgDB)
		userRepo = postgres.NewUserRepository(pgDB)
		attendanceRepo = postgres.NewAttendanceRepository(pgDB)
		enrollmentRepo = postgres.NewEnrollmentRepository(pgDB)
		auditRepo = postgres.NewAuditRepository(pgDB)
	} else {
		db, err := sqlite.NewSQLiteDB(dbPath)
		if err != nil {
			log.Fatalf("Failed to initialize database: %v", err)
		}
		defer db.Close()
		log.Printf("📦 SQLite database ready at: %s (Set DB_DIALECT=postgres to use PostgreSQL)", dbPath)

		adminRepo = sqlite.NewAdminRepository(db)
		deviceRepo = sqlite.NewDeviceRepository(db)
		userRepo = sqlite.NewUserRepository(db)
		attendanceRepo = sqlite.NewAttendanceRepository(db)
		enrollmentRepo = sqlite.NewEnrollmentRepository(db)
		auditRepo = sqlite.NewAuditRepository(db)
	}

	// 3. Infrastructure: Security Token Maker
	tokenMaker := token.NewTokenMaker(jwtSecret)

	// 4. Core: Use Cases
	authUC := usecase.NewAuthUseCase(adminRepo, auditRepo, tokenMaker)
	deviceUC := usecase.NewDeviceUseCase(deviceRepo, auditRepo)
	userUC := usecase.NewUserUseCase(userRepo, auditRepo)
	attendanceUC := usecase.NewAttendanceUseCase(attendanceRepo)
	syncUC := usecase.NewSyncUseCase(deviceRepo, userRepo, attendanceUC, enrollmentRepo)
	enrollmentUC := usecase.NewEnrollmentUseCase(enrollmentRepo, userRepo, uploadDir)
	auditUC := usecase.NewAuditUseCase(auditRepo)

	// 5. Adapters: HTTP Handlers
	handlers := &http.Handlers{
		Auth:       http.NewAuthHandler(authUC),
		User:       http.NewUserHandler(userUC),
		Attendance: http.NewAttendanceHandler(attendanceUC),
		Device:     http.NewDeviceHandler(deviceUC),
		Sync:       http.NewSyncHandler(syncUC),
		Enrollment: http.NewEnrollmentHandler(enrollmentUC),
		Audit:      http.NewAuditHandler(auditUC),
	}

	// 6. Router Setup
	router := http.SetupRouter(handlers, tokenMaker, deviceUC)

	log.Printf("Server listening on :%s", port)
	if err := router.Run(":" + port); err != nil {
		log.Fatalf("Server failed to run: %v", err)
	}
}
