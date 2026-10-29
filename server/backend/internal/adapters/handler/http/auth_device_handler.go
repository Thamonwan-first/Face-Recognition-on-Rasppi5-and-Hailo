package http

import (
	"net/http"

	"attendance-backend/internal/pkg/token"
	"attendance-backend/internal/usecase"
	"github.com/gin-gonic/gin"
)

type AuthHandler struct {
	authUC *usecase.AuthUseCase
}

func NewAuthHandler(authUC *usecase.AuthUseCase) *AuthHandler {
	return &AuthHandler{authUC: authUC}
}

type loginRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

func (h *AuthHandler) Login(c *gin.Context) {
	var req loginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	resp, err := h.authUC.Login(c.Request.Context(), req.Username, req.Password)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid username or password"})
		return
	}

	c.JSON(http.StatusOK, resp)
}

func (h *AuthHandler) Me(c *gin.Context) {
	claimsVal, exists := c.Get(ContextAdminKey)
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}
	claims := claimsVal.(*token.Claims)

	admin, err := h.authUC.GetCurrentAdmin(c.Request.Context(), claims.AdminID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Admin not found"})
		return
	}

	c.JSON(http.StatusOK, admin)
}

// ---------------- Device Handler ----------------

type DeviceHandler struct {
	deviceUC *usecase.DeviceUseCase
}

func NewDeviceHandler(deviceUC *usecase.DeviceUseCase) *DeviceHandler {
	return &DeviceHandler{deviceUC: deviceUC}
}

func (h *DeviceHandler) List(c *gin.Context) {
	devices, err := h.deviceUC.List(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, devices)
}

func (h *DeviceHandler) Register(c *gin.Context) {
	var dto usecase.DeviceDTO
	if err := c.ShouldBindJSON(&dto); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	claims := c.MustGet(ContextAdminKey).(*token.Claims)
	resp, err := h.deviceUC.Register(c.Request.Context(), claims.AdminID, &dto)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, resp)
}

func (h *DeviceHandler) Delete(c *gin.Context) {
	id := c.Param("id")
	claims := c.MustGet(ContextAdminKey).(*token.Claims)
	if err := h.deviceUC.Delete(c.Request.Context(), claims.AdminID, id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "deleted"})
}
