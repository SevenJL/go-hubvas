package handler

import (
	"strconv"

	"github.com/gin-gonic/gin"
	appCommunity "github.com/hubvas/internal/application/community"
	"github.com/hubvas/internal/domain/canvas"
	"github.com/hubvas/internal/domain/identity"
	"github.com/hubvas/internal/interfaces/http/response"
)

// CommunityHandler handles community-related HTTP requests.
type CommunityHandler struct {
	appSvc *appCommunity.CommunityApplicationService
}

// NewCommunityHandler creates a new CommunityHandler.
func NewCommunityHandler(appSvc *appCommunity.CommunityApplicationService) *CommunityHandler {
	return &CommunityHandler{appSvc: appSvc}
}

// Browse handles GET /api/community.
func (h *CommunityHandler) Browse(c *gin.Context) {
	var req appCommunity.SearchRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	feed, err := h.appSvc.Browse(c.Request.Context(), req)
	if err != nil {
		response.Error(c, 500, "browse_failed", err.Error())
		return
	}
	response.OK(c, feed)
}

// Like handles POST /api/canvases/:id/like.
func (h *CommunityHandler) Like(c *gin.Context) {
	userID := getUserID(c)
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "invalid canvas ID")
		return
	}

	if err := h.appSvc.LikeCanvas(c.Request.Context(), canvas.CanvasID(id), identity.UserID(userID)); err != nil {
		response.Error(c, 400, "like_failed", err.Error())
		return
	}
	response.OK(c, gin.H{"status": "liked"})
}

// Unlike handles DELETE /api/canvases/:id/like.
func (h *CommunityHandler) Unlike(c *gin.Context) {
	userID := getUserID(c)
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "invalid canvas ID")
		return
	}

	if err := h.appSvc.UnlikeCanvas(c.Request.Context(), canvas.CanvasID(id), identity.UserID(userID)); err != nil {
		response.Error(c, 400, "unlike_failed", err.Error())
		return
	}
	response.OK(c, gin.H{"status": "unliked"})
}

// PostComment handles POST /api/canvases/:id/comments.
func (h *CommunityHandler) PostComment(c *gin.Context) {
	userID := getUserID(c)
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "invalid canvas ID")
		return
	}

	var req appCommunity.NewCommentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	comment, err := h.appSvc.PostComment(c.Request.Context(), canvas.CanvasID(id), identity.UserID(userID), req)
	if err != nil {
		response.Error(c, 400, "comment_failed", err.Error())
		return
	}
	response.Created(c, comment)
}

// GetComments handles GET /api/canvases/:id/comments.
func (h *CommunityHandler) GetComments(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "invalid canvas ID")
		return
	}

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))

	comments, total, err := h.appSvc.GetComments(c.Request.Context(), canvas.CanvasID(id), page, pageSize)
	if err != nil {
		response.Error(c, 500, "get_comments_failed", err.Error())
		return
	}
	response.OK(c, gin.H{
		"items":      comments,
		"total":      total,
		"page":       page,
		"page_size":  pageSize,
	})
}
