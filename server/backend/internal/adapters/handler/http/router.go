package http

import (
	"net/http"
	"time"

	"attendance-backend/internal/pkg/token"
	"attendance-backend/internal/usecase"
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

type Handlers struct {
	Auth       *AuthHandler
	User       *UserHandler
	Attendance *AttendanceHandler
	Device     *DeviceHandler
	Sync       *SyncHandler
	Enrollment *EnrollmentHandler
	Audit      *AuditHandler
}

func SetupRouter(handlers *Handlers, tokenMaker *token.TokenMaker, deviceUC *usecase.DeviceUseCase) *gin.Engine {
	r := gin.Default()

	// CORS Setup
	r.Use(cors.New(cors.Config{
		AllowOrigins:     []string{"*"},
		AllowMethods:     []string{"GET", "POST", "PUT", "PATCH", "DELETE", "HEAD", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Length", "Content-Type", "Authorization", "X-API-Key"},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: true,
		MaxAge:           12 * time.Hour,
	}))

	// Health check
	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok", "service": "attendance-backend-go", "architecture": "clean-architecture"})
	})

	// Public Auth endpoints
	authGroup := r.Group("/auth")
	{
		authGroup.POST("/login", handlers.Auth.Login)
	}
	r.POST("/api/auth/login", handlers.Auth.Login)

	// Protected Admin Routes (/api)
	adminAuth := AuthMiddleware(tokenMaker)
	api := r.Group("/api")
	{
		// Auth profile
		api.GET("/auth/me", adminAuth, handlers.Auth.Me)

		// Users
		users := api.Group("/users")
		users.Use(adminAuth)
		{
			users.GET("", handlers.User.List)
			users.POST("", handlers.User.Create)
			users.GET("/:id", handlers.User.Get)
			users.PUT("/:id", handlers.User.Update)
			users.DELETE("/:id", handlers.User.Delete)
		}

		// Attendance
		attendance := api.Group("/attendance")
		attendance.Use(adminAuth)
		{
			attendance.GET("/pairs", handlers.Attendance.ListPairs)
			attendance.GET("/summary", handlers.Attendance.ListPairs)
			attendance.GET("/events", handlers.Attendance.ListEvents)
		}

		// Devices
		devices := api.Group("/devices")
		devices.Use(adminAuth)
		{
			devices.GET("", handlers.Device.List)
			devices.GET("/status", handlers.Device.List)
			devices.POST("", handlers.Device.Register)
			devices.DELETE("/:id", handlers.Device.Delete)
		}

		// Enrollment (Dashboard admin upload)
		enrollment := api.Group("/enrollment")
		enrollment.Use(adminAuth)
		{
			enrollment.GET("/pending", handlers.Enrollment.Pending)
			enrollment.POST("", handlers.Enrollment.Create)
		}

		// Audit Log
		audit := api.Group("/audit")
		audit.Use(adminAuth)
		{
			audit.GET("", handlers.Audit.List)
		}
	}

	// Device Sync Endpoints (Protected by Device X-API-Key)
	deviceAuth := DeviceAuthMiddleware(deviceUC)

	registerSyncRoutes := func(g *gin.RouterGroup) {
		g.Use(deviceAuth)
		g.POST("/events", handlers.Sync.SyncEvents)
		g.POST("/heartbeat", handlers.Sync.Heartbeat)
		g.GET("/users", handlers.Sync.SyncUsers)
		g.POST("/enrollment", handlers.Sync.KioskEnrollment)
		g.GET("/enrollment/claim", handlers.Enrollment.Claim)
		g.POST("/enrollment/result", handlers.Enrollment.Result)
	}

	// Mount on /sync and /api/sync for Kiosk compatibility
	syncGroup := r.Group("/sync")
	registerSyncRoutes(syncGroup)

	apiSyncGroup := r.Group("/api/sync")
	registerSyncRoutes(apiSyncGroup)

	return r
}
