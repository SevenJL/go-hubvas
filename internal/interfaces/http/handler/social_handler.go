package handler

import (
	"errors"
	"github.com/gin-gonic/gin"
	appsocial "github.com/hubvas/internal/application/social"
	"github.com/hubvas/internal/domain/identity"
	"github.com/hubvas/internal/domain/shared"
	"github.com/hubvas/internal/interfaces/http/middleware"
	"github.com/hubvas/internal/interfaces/http/response"
	"strconv"
)

type SocialHandler struct{ svc *appsocial.Service }

func NewSocialHandler(s *appsocial.Service) *SocialHandler { return &SocialHandler{svc: s} }
func page(c *gin.Context) (int, int) {
	p, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	s, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	return p, s
}
func idParam(c *gin.Context, name string) (identity.UserID, bool) {
	v, err := strconv.ParseInt(func() string {
		if v := c.Param(name); v != "" {
			return v
		}
		return c.Param("identifier")
	}(), 10, 64)
	if err != nil {
		response.BadRequest(c, "invalid user ID")
		return 0, false
	}
	return identity.UserID(v), true
}
func publicDomainErrorMessage(err error) string {
	var domainErr *shared.DomainError
	if errors.As(err, &domainErr) && domainErr.Message != "" {
		return domainErr.Message
	}
	return "request failed"
}

func socialError(c *gin.Context, err error) {
	message := publicDomainErrorMessage(err)
	switch {
	case errors.Is(err, shared.ErrInvalidArgument):
		response.BadRequest(c, message)
	case errors.Is(err, shared.ErrUnauthorized):
		response.Unauthorized(c, message)
	case errors.Is(err, shared.ErrForbidden):
		response.Forbidden(c, message)
	case errors.Is(err, shared.ErrNotFound):
		response.NotFound(c, message)
	case errors.Is(err, shared.ErrAlreadyExists), errors.Is(err, shared.ErrConflict):
		response.Conflict(c, message)
	case errors.Is(err, shared.ErrLimitExceeded):
		response.Error(c, 429, "rate_limited", message)
	default:
		response.InternalError(c, message)
	}
}
func (h *SocialHandler) Profile(c *gin.Context) {
	v, err := h.svc.Profile(c.Request.Context(), c.Param("identifier"), middleware.GetUserID(c))
	if err != nil {
		socialError(c, err)
		return
	}
	response.OK(c, v)
}
func (h *SocialHandler) UserCanvases(c *gin.Context) {
	p, s := page(c)
	v, err := h.svc.PublishedByUser(c.Request.Context(), c.Param("identifier"), middleware.GetUserID(c), p, s)
	if err != nil {
		socialError(c, err)
		return
	}
	response.OK(c, v)
}
func (h *SocialHandler) FollowingFeed(c *gin.Context) {
	p, s := page(c)
	v, err := h.svc.FollowingFeed(c.Request.Context(), middleware.GetUserID(c), p, s)
	if err != nil {
		socialError(c, err)
		return
	}
	response.OK(c, v)
}
func (h *SocialHandler) Follow(c *gin.Context) {
	id, ok := idParam(c, "id")
	if !ok {
		return
	}
	if err := h.svc.Follow(c.Request.Context(), middleware.GetUserID(c), id); err != nil {
		socialError(c, err)
		return
	}
	response.OK(c, gin.H{"following": true})
}
func (h *SocialHandler) Unfollow(c *gin.Context) {
	id, ok := idParam(c, "id")
	if !ok {
		return
	}
	if err := h.svc.Unfollow(c.Request.Context(), middleware.GetUserID(c), id); err != nil {
		socialError(c, err)
		return
	}
	response.OK(c, gin.H{"following": false})
}
func (h *SocialHandler) Followers(c *gin.Context) { h.relationships(c, "followers") }
func (h *SocialHandler) Following(c *gin.Context) { h.relationships(c, "following") }
func (h *SocialHandler) relationships(c *gin.Context, kind string) {
	id, ok := idParam(c, "id")
	if !ok {
		return
	}
	p, s := page(c)
	v, err := h.svc.Relationships(c.Request.Context(), middleware.GetUserID(c), id, kind, p, s)
	if err != nil {
		socialError(c, err)
		return
	}
	response.OK(c, v)
}
func (h *SocialHandler) Block(c *gin.Context) {
	id, ok := idParam(c, "id")
	if !ok {
		return
	}
	if err := h.svc.Block(c.Request.Context(), middleware.GetUserID(c), id); err != nil {
		socialError(c, err)
		return
	}
	response.OK(c, gin.H{"blocked": true})
}
func (h *SocialHandler) Unblock(c *gin.Context) {
	id, ok := idParam(c, "id")
	if !ok {
		return
	}
	if err := h.svc.Unblock(c.Request.Context(), middleware.GetUserID(c), id); err != nil {
		socialError(c, err)
		return
	}
	response.OK(c, gin.H{"blocked": false})
}
func (h *SocialHandler) Blocks(c *gin.Context) {
	p, s := page(c)
	v, err := h.svc.Blocks(c.Request.Context(), middleware.GetUserID(c), p, s)
	if err != nil {
		socialError(c, err)
		return
	}
	response.OK(c, v)
}
func (h *SocialHandler) Notifications(c *gin.Context) {
	p, s := page(c)
	v, err := h.svc.Notifications(c.Request.Context(), middleware.GetUserID(c), p, s)
	if err != nil {
		socialError(c, err)
		return
	}
	response.OK(c, v)
}
func (h *SocialHandler) UnreadCount(c *gin.Context) {
	v, err := h.svc.UnreadCount(c.Request.Context(), middleware.GetUserID(c))
	if err != nil {
		socialError(c, err)
		return
	}
	response.OK(c, gin.H{"count": v})
}
func (h *SocialHandler) MarkRead(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "invalid notification ID")
		return
	}
	if err = h.svc.MarkRead(c.Request.Context(), middleware.GetUserID(c), id); err != nil {
		socialError(c, err)
		return
	}
	response.OK(c, gin.H{"read": true})
}
func (h *SocialHandler) MarkAllRead(c *gin.Context) {
	if err := h.svc.MarkAllRead(c.Request.Context(), middleware.GetUserID(c)); err != nil {
		socialError(c, err)
		return
	}
	response.OK(c, gin.H{"read": true})
}
func (h *SocialHandler) Report(c *gin.Context) {
	var in appsocial.ReportRequest
	if err := c.ShouldBindJSON(&in); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	v, err := h.svc.CreateReport(c.Request.Context(), middleware.GetUserID(c), in)
	if err != nil {
		socialError(c, err)
		return
	}
	response.Created(c, v)
}
func (h *SocialHandler) AdminReports(c *gin.Context) {
	p, s := page(c)
	items, total, err := h.svc.Reports(c.Request.Context(), middleware.GetUserID(c), c.Query("status"), p, s)
	if err != nil {
		socialError(c, err)
		return
	}
	response.OK(c, gin.H{"items": items, "total": total, "page": p, "page_size": s})
}
func (h *SocialHandler) ReviewReport(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "invalid report ID")
		return
	}
	var in appsocial.ReviewReportRequest
	if err = c.ShouldBindJSON(&in); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	v, err := h.svc.ReviewReport(c.Request.Context(), middleware.GetUserID(c), id, in)
	if err != nil {
		socialError(c, err)
		return
	}
	response.OK(c, v)
}
func (h *SocialHandler) UserStatus(c *gin.Context) {
	id, ok := idParam(c, "id")
	if !ok {
		return
	}
	var in appsocial.UserStatusRequest
	if err := c.ShouldBindJSON(&in); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	if err := h.svc.SetUserStatus(c.Request.Context(), middleware.GetUserID(c), id, in.Status); err != nil {
		socialError(c, err)
		return
	}
	response.OK(c, gin.H{"status": in.Status})
}
func (h *SocialHandler) ModerateComment(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "invalid comment ID")
		return
	}
	var in appsocial.ModerationRequest
	if err = c.ShouldBindJSON(&in); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	if err = h.svc.ModerateComment(c.Request.Context(), middleware.GetUserID(c), id, in.Status); err != nil {
		socialError(c, err)
		return
	}
	response.OK(c, gin.H{"status": in.Status})
}
func (h *SocialHandler) ModerateCanvas(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "invalid canvas ID")
		return
	}
	var in appsocial.ModerationRequest
	if err = c.ShouldBindJSON(&in); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	if err = h.svc.ModerateCanvas(c.Request.Context(), middleware.GetUserID(c), id, in.Status); err != nil {
		socialError(c, err)
		return
	}
	response.OK(c, gin.H{"status": in.Status})
}

func (h *SocialHandler) AdminAuditLogs(c *gin.Context) {
	p, s := page(c)
	items, total, err := h.svc.AuditLogs(c.Request.Context(), middleware.GetUserID(c), p, s)
	if err != nil {
		socialError(c, err)
		return
	}
	response.OK(c, gin.H{"items": items, "total": total, "page": p, "page_size": s})
}

func (h *SocialHandler) ReplayNotificationOutbox(c *gin.Context) {
	var input struct {
		Limit int `json:"limit" binding:"omitempty,min=1,max=1000"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	count, err := h.svc.ReplayNotificationOutbox(c.Request.Context(), middleware.GetUserID(c), input.Limit)
	if err != nil {
		socialError(c, err)
		return
	}
	response.OK(c, gin.H{"replayed": count})
}
