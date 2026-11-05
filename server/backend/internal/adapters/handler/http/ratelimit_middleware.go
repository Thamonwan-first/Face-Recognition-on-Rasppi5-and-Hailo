package http

import (
	"net/http"

	"attendance-backend/internal/pkg/ratelimit"
	"github.com/gin-gonic/gin"
)

// RateLimitMiddleware applies in-memory token bucket rate limiting per client IP
func RateLimitMiddleware(limiter *ratelimit.TokenBucketLimiter) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Device sync endpoints with valid API key bypass general dashboard rate limit
		if c.GetHeader("X-API-Key") != "" {
			c.Next()
			return
		}

		clientIP := c.ClientIP()
		if !limiter.Allow(clientIP) {
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
				"error":   "Too Many Requests",
				"message": "คำขอถี่เกินไป กรุณารอสักครู่แล้วลองใหม่อีกครั้ง",
			})
			return
		}

		c.Next()
	}
}
