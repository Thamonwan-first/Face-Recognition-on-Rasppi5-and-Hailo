package http

import (
	"encoding/json"
	"io"
	"time"

	"attendance-backend/internal/pkg/broker"
	"github.com/gin-gonic/gin"
)

type RealtimeHandler struct {
	broker *broker.EventBroker
}

func NewRealtimeHandler(b *broker.EventBroker) *RealtimeHandler {
	return &RealtimeHandler{broker: b}
}

// Stream streams server-sent events to authenticated clients
func (h *RealtimeHandler) Stream(c *gin.Context) {
	c.Writer.Header().Set("Content-Type", "text/event-stream")
	c.Writer.Header().Set("Cache-Control", "no-cache")
	c.Writer.Header().Set("Connection", "keep-alive")
	c.Writer.Header().Set("Transfer-Encoding", "chunked")

	ch := h.broker.Subscribe()
	defer h.broker.Unsubscribe(ch)

	// Initial handshake
	c.SSEvent("connected", gin.H{
		"status": "connected",
		"time":   time.Now().Unix(),
	})
	c.Writer.Flush()

	ticker := time.NewTicker(20 * time.Second) // keep-alive heartbeat
	defer ticker.Stop()

	c.Stream(func(w io.Writer) bool {
		select {
		case <-c.Request.Context().Done():
			return false
		case <-ticker.C:
			c.SSEvent("ping", gin.H{"ping": time.Now().Unix()})
			return true
		case evt, ok := <-ch:
			if !ok {
				return false
			}
			payloadBytes, err := json.Marshal(evt.Payload)
			if err != nil {
				c.SSEvent(evt.Type, "{}")
			} else {
				c.SSEvent(evt.Type, string(payloadBytes))
			}
			return true
		}
	})
}
