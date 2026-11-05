package http

import (
	"net/http"

	"attendance-backend/internal/entity"
	"attendance-backend/internal/pkg/broker"
	"attendance-backend/internal/pkg/token"
	"attendance-backend/internal/usecase"
	"github.com/gin-gonic/gin"
)

type SettingsHandler struct {
	settingsUC *usecase.SettingsUseCase
	broker     *broker.EventBroker
}

func NewSettingsHandler(settingsUC *usecase.SettingsUseCase, b ...*broker.EventBroker) *SettingsHandler {
	var brokerInst *broker.EventBroker
	if len(b) > 0 {
		brokerInst = b[0]
	}
	return &SettingsHandler{settingsUC: settingsUC, broker: brokerInst}
}

func (h *SettingsHandler) Get(c *gin.Context) {
	settings, err := h.settingsUC.Get(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, settings)
}

func (h *SettingsHandler) Update(c *gin.Context) {
	claimsVal, exists := c.Get(ContextAdminKey)
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}
	claims := claimsVal.(*token.Claims)

	var dto entity.SystemSettings
	if err := c.ShouldBindJSON(&dto); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.settingsUC.Update(c.Request.Context(), claims.AdminID, &dto); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if h.broker != nil {
		h.broker.Publish(broker.Event{
			Type:    "settings",
			Payload: gin.H{"action": "updated"},
		})
		h.broker.Publish(broker.Event{
			Type:    "attendance",
			Payload: gin.H{"action": "settings_updated"},
		})
	}

	c.JSON(http.StatusOK, gin.H{"status": "success", "message": "บันทึกการตั้งค่าระบบเรียบร้อยแล้ว"})
}
