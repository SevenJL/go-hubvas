package handler

import (
	"github.com/gin-gonic/gin"
	"github.com/hubvas/internal/application/auth"
	"github.com/hubvas/internal/domain/identity"
	"github.com/hubvas/internal/interfaces/http/response"
)

// AuthHandler handles authentication-related HTTP requests.
type AuthHandler struct {
	appSvc *auth.AuthApplicationService
}

// NewAuthHandler creates a new AuthHandler.
func NewAuthHandler(appSvc *auth.AuthApplicationService) *AuthHandler {
	return &AuthHandler{appSvc: appSvc}
}

// Register handles POST /api/auth/register.
// On success, the user is created AND tokens are returned so the client
// can immediately authenticate without a second login request.
func (h *AuthHandler) Register(c *gin.Context) {
	var req auth.RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	result, err := h.appSvc.Register(c.Request.Context(), req)
	if err != nil {
		response.Error(c, 400, "register_failed", err.Error())
		return
	}

	response.Created(c, result)
}

// Login handles POST /api/auth/login.
func (h *AuthHandler) Login(c *gin.Context) {
	var req auth.LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	tokens, err := h.appSvc.Login(c.Request.Context(), req)
	if err != nil {
		response.Unauthorized(c, err.Error())
		return
	}
	response.OK(c, tokens)
}

// Refresh handles POST /api/auth/refresh.
// Accepts a refresh token and returns a new token pair.
func (h *AuthHandler) Refresh(c *gin.Context) {
	var req auth.RefreshRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	tokens, err := h.appSvc.Refresh(c.Request.Context(), req)
	if err != nil {
		response.Unauthorized(c, err.Error())
		return
	}
	response.OK(c, tokens)
}

// UpdateProfile handles PUT /api/auth/profile.
func (h *AuthHandler) UpdateProfile(c *gin.Context) {
	userIDVal, exists := c.Get("userID")
	if !exists {
		response.Unauthorized(c, "not authenticated")
		return
	}

	var req auth.UpdateProfileRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	userID := userIDVal.(identity.UserID)
	user, err := h.appSvc.UpdateProfile(c.Request.Context(), userID, req)
	if err != nil {
		response.Error(c, 400, "update_failed", err.Error())
		return
	}
	response.OK(c, user)
}

// Me handles GET /api/auth/me.
// The userID is injected by AuthMiddleware.
func (h *AuthHandler) Me(c *gin.Context) {
	userIDVal, exists := c.Get("userID")
	if !exists {
		response.Unauthorized(c, "not authenticated")
		return
	}

	userID := userIDVal.(identity.UserID)
	user, err := h.appSvc.GetUser(c.Request.Context(), userID)
	if err != nil {
		response.NotFound(c, "user not found")
		return
	}
	response.OK(c, user)
}
