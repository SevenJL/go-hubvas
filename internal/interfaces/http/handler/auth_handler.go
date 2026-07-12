package handler

import (
	"errors"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	appauth "github.com/hubvas/internal/application/auth"
	"github.com/hubvas/internal/domain/identity"
	"github.com/hubvas/internal/domain/shared"
	"github.com/hubvas/internal/interfaces/http/response"
	"github.com/hubvas/pkg/config"
)

type AuthHandler struct {
	appSvc *appauth.AuthApplicationService
	cfg    config.AuthConfig
}

func NewAuthHandler(appSvc *appauth.AuthApplicationService, cfg config.AuthConfig) *AuthHandler {
	return &AuthHandler{appSvc: appSvc, cfg: cfg}
}

func (h *AuthHandler) metadata(c *gin.Context) appauth.SessionMetadata {
	return appauth.SessionMetadata{UserAgent: c.Request.UserAgent(), IPAddress: c.ClientIP()}
}

func (h *AuthHandler) sameSite() http.SameSite {
	switch strings.ToLower(h.cfg.CookieSameSite) {
	case "none":
		return http.SameSiteNoneMode
	case "lax":
		return http.SameSiteLaxMode
	default:
		return http.SameSiteStrictMode
	}
}

func (h *AuthHandler) setRefreshCookie(c *gin.Context, value string) {
	http.SetCookie(c.Writer, &http.Cookie{
		Name: h.cfg.CookieName, Value: value, Path: "/api/auth", Domain: h.cfg.CookieDomain,
		MaxAge: int(h.cfg.RefreshTokenTTL / time.Second), Expires: time.Now().Add(h.cfg.RefreshTokenTTL),
		HttpOnly: true, Secure: h.cfg.CookieSecure, SameSite: h.sameSite(),
	})
}

func (h *AuthHandler) clearRefreshCookie(c *gin.Context) {
	http.SetCookie(c.Writer, &http.Cookie{
		Name: h.cfg.CookieName, Value: "", Path: "/api/auth", Domain: h.cfg.CookieDomain,
		MaxAge: -1, Expires: time.Unix(1, 0), HttpOnly: true, Secure: h.cfg.CookieSecure, SameSite: h.sameSite(),
	})
}

func (h *AuthHandler) refreshToken(c *gin.Context) string {
	if cookie, err := c.Cookie(h.cfg.CookieName); err == nil {
		return cookie
	}
	var req appauth.RefreshRequest
	_ = c.ShouldBindJSON(&req)
	return req.RefreshToken
}

// trustedCookieOrigin rejects browser-initiated cross-origin requests to
// cookie-authenticated endpoints. Requests without Origin remain available to
// non-browser clients and cannot exploit ambient browser cookies.
func trustedCookieOrigin(r *http.Request) bool {
	origin := r.Header.Get("Origin")
	if origin == "" {
		return true
	}
	parsed, err := url.Parse(origin)
	if err != nil || parsed.Host == "" || parsed.Host != r.Host {
		return false
	}
	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}
	if forwarded := strings.TrimSpace(strings.Split(r.Header.Get("X-Forwarded-Proto"), ",")[0]); forwarded != "" {
		scheme = forwarded
	}
	return parsed.Scheme == scheme
}

func authenticatedUserID(c *gin.Context) (identity.UserID, bool) {
	value, exists := c.Get("userID")
	if !exists {
		return 0, false
	}
	id, ok := value.(identity.UserID)
	return id, ok && id > 0
}

func respondAuthError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, shared.ErrInvalidArgument):
		response.BadRequest(c, err.Error())
	case errors.Is(err, shared.ErrUnauthorized):
		response.Unauthorized(c, err.Error())
	case errors.Is(err, shared.ErrForbidden):
		response.Forbidden(c, err.Error())
	case errors.Is(err, shared.ErrNotFound):
		response.NotFound(c, err.Error())
	case errors.Is(err, shared.ErrAlreadyExists), errors.Is(err, shared.ErrConflict):
		response.Conflict(c, err.Error())
	case errors.Is(err, shared.ErrLimitExceeded):
		response.Error(c, http.StatusTooManyRequests, "rate_limited", err.Error())
	default:
		response.InternalError(c, "authentication service unavailable")
	}
}

func (h *AuthHandler) Register(c *gin.Context) {
	var req appauth.RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	result, err := h.appSvc.Register(c.Request.Context(), req, h.metadata(c))
	if err != nil {
		respondAuthError(c, err)
		return
	}
	h.setRefreshCookie(c, result.Tokens.RefreshToken)
	response.Created(c, result)
}
func (h *AuthHandler) Login(c *gin.Context) {
	var req appauth.LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	tokens, err := h.appSvc.Login(c.Request.Context(), req, h.metadata(c))
	if err != nil {
		respondAuthError(c, err)
		return
	}
	h.setRefreshCookie(c, tokens.RefreshToken)
	response.OK(c, tokens)
}
func (h *AuthHandler) Refresh(c *gin.Context) {
	if !trustedCookieOrigin(c.Request) {
		response.Error(c, http.StatusForbidden, "invalid_origin", "cross-origin refresh is not allowed")
		return
	}
	tokens, err := h.appSvc.Refresh(c.Request.Context(), h.refreshToken(c), h.metadata(c))
	if err != nil {
		h.clearRefreshCookie(c)
		respondAuthError(c, err)
		return
	}
	h.setRefreshCookie(c, tokens.RefreshToken)
	response.OK(c, tokens)
}
func (h *AuthHandler) Logout(c *gin.Context) {
	if !trustedCookieOrigin(c.Request) {
		response.Error(c, http.StatusForbidden, "invalid_origin", "cross-origin logout is not allowed")
		return
	}
	token := h.refreshToken(c)
	_ = h.appSvc.Logout(c.Request.Context(), token)
	h.clearRefreshCookie(c)
	response.OK(c, gin.H{"logged_out": true})
}
func (h *AuthHandler) LogoutAll(c *gin.Context) {
	userID, ok := authenticatedUserID(c)
	if !ok {
		response.Unauthorized(c, "not authenticated")
		return
	}
	if err := h.appSvc.LogoutAll(c.Request.Context(), userID); err != nil {
		response.InternalError(c, "logout failed")
		return
	}
	h.clearRefreshCookie(c)
	response.OK(c, gin.H{"logged_out": true})
}
func (h *AuthHandler) UpdateProfile(c *gin.Context) {
	userID, ok := authenticatedUserID(c)
	if !ok {
		response.Unauthorized(c, "not authenticated")
		return
	}
	var req appauth.UpdateProfileRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	user, err := h.appSvc.UpdateProfile(c.Request.Context(), userID, req)
	if err != nil {
		respondAuthError(c, err)
		return
	}
	response.OK(c, user)
}
func (h *AuthHandler) Me(c *gin.Context) {
	userID, ok := authenticatedUserID(c)
	if !ok {
		response.Unauthorized(c, "not authenticated")
		return
	}
	user, err := h.appSvc.GetUser(c.Request.Context(), userID)
	if err != nil {
		response.NotFound(c, "user not found")
		return
	}
	response.OK(c, user)
}
