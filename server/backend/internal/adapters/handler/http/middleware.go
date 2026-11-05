package http

import (
	"net/http"
	"strings"

	"attendance-backend/internal/pkg/token"
	"attendance-backend/internal/usecase"
	"github.com/gin-gonic/gin"
)

const (
	ContextAdminKey  = "admin_claims"
	ContextDeviceKey = "authenticated_device"
)

func AuthMiddleware(tokenMaker *token.TokenMaker) gin.HandlerFunc {
	return func(c *gin.Context) {
		var accessToken string
		authHeader := c.GetHeader("Authorization")
		if authHeader != "" {
			fields := strings.Fields(authHeader)
			if len(fields) == 2 && strings.ToLower(fields[0]) == "bearer" {
				accessToken = fields[1]
			} else {
				c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Invalid authorization header format"})
				return
			}
		} else {
			// Support query parameter ?token= for SSE (EventSource) connections
			accessToken = c.Query("token")
		}

		if accessToken == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Authorization header or token parameter required"})
			return
		}

		claims, err := tokenMaker.VerifyToken(accessToken)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Invalid or expired token"})
			return
		}

		c.Set(ContextAdminKey, claims)
		c.Next()
	}
}

func RequirePermission(perms ...string) gin.HandlerFunc {
	return func(c *gin.Context) {
		claimsVal, exists := c.Get(ContextAdminKey)
		if !exists {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
			return
		}
		claims := claimsVal.(*token.Claims)

		// Super Admin passes all
		if claims.Role == "admin" {
			c.Next()
			return
		}

		// Check if user has '*' or any of the required permissions
		for _, userPerm := range claims.Permissions {
			if userPerm == "*" {
				c.Next()
				return
			}
			for _, requiredPerm := range perms {
				if userPerm == requiredPerm {
					c.Next()
					return
				}
			}
		}

		c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "ไม่มีสิทธิ์เข้าถึงส่วนนี้"})
	}
}

func DeviceAuthMiddleware(deviceUC *usecase.DeviceUseCase) gin.HandlerFunc {
	return func(c *gin.Context) {
		apiKey := c.GetHeader("X-API-Key")
		if apiKey == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "X-API-Key header required"})
			return
		}

		device, err := deviceUC.AuthenticateDevice(c.Request.Context(), apiKey)
		if err != nil || device == nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Invalid or revoked device API key"})
			return
		}

		c.Set(ContextDeviceKey, device)
		if clientIP := c.ClientIP(); clientIP != "" && device.IPAddress != clientIP {
			device.IPAddress = clientIP
			_ = deviceUC.UpdateIP(c.Request.Context(), device.ID, clientIP)
		}
		c.Next()
	}
}
