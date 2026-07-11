package handler

import (
	"errors"
	"strconv"

	"github.com/gin-gonic/gin"
	appCanvas "github.com/hubvas/internal/application/canvas"
	"github.com/hubvas/internal/domain/canvas"
	"github.com/hubvas/internal/domain/identity"
	"github.com/hubvas/internal/domain/shared"
	"github.com/hubvas/internal/interfaces/http/middleware"
	"github.com/hubvas/internal/interfaces/http/response"
)

// CanvasHandler handles canvas-related HTTP requests.
type CanvasHandler struct {
	appSvc *appCanvas.CanvasApplicationService
}

// NewCanvasHandler creates a new CanvasHandler.
func NewCanvasHandler(appSvc *appCanvas.CanvasApplicationService) *CanvasHandler {
	return &CanvasHandler{appSvc: appSvc}
}

// Create handles POST /api/canvases.
func (h *CanvasHandler) Create(c *gin.Context) {
	userID := getUserID(c)

	var req appCanvas.CreateCanvasRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	dto, err := h.appSvc.Create(c.Request.Context(), identity.UserID(userID), req)
	if err != nil {
		response.Error(c, 400, "create_failed", err.Error())
		return
	}
	response.Created(c, dto)
}

// Get handles GET /api/canvases/:id.
func (h *CanvasHandler) Get(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "invalid canvas ID")
		return
	}

	requesterID := middleware.GetUserID(c)
	dto, err := h.appSvc.Get(c.Request.Context(), canvas.CanvasID(id), requesterID)
	if err != nil {
		if errors.Is(err, shared.ErrForbidden) {
			response.Forbidden(c, "you do not have access to this canvas")
		} else {
			response.NotFound(c, "canvas not found")
		}
		return
	}
	response.OK(c, dto)
}

// ListMine handles GET /api/canvases (lists the authenticated user's canvases).
func (h *CanvasHandler) ListMine(c *gin.Context) {
	userID := getUserID(c)

	dtos, err := h.appSvc.ListByOwner(c.Request.Context(), identity.UserID(userID))
	if err != nil {
		response.Error(c, 500, "list_failed", err.Error())
		return
	}
	response.OK(c, dtos)
}

// ListShared handles GET /api/canvases/shared.
func (h *CanvasHandler) ListShared(c *gin.Context) {
	userID := identity.UserID(getUserID(c))
	dtos, err := h.appSvc.ListShared(c.Request.Context(), userID)
	if err != nil {
		response.InternalError(c, err.Error())
		return
	}
	response.OK(c, dtos)
}

// ListMembers handles GET /api/canvases/:id/members.
func (h *CanvasHandler) ListMembers(c *gin.Context) {
	canvasID, ok := parseCanvasID(c)
	if !ok {
		return
	}
	members, err := h.appSvc.ListMembers(c.Request.Context(), canvasID, identity.UserID(getUserID(c)))
	if err != nil {
		respondCanvasError(c, err, "list_members_failed")
		return
	}
	response.OK(c, members)
}

// AddMember handles POST /api/canvases/:id/members.
func (h *CanvasHandler) AddMember(c *gin.Context) {
	canvasID, ok := parseCanvasID(c)
	if !ok {
		return
	}
	var req appCanvas.AddMemberRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	member, err := h.appSvc.AddMember(c.Request.Context(), canvasID, identity.UserID(getUserID(c)), req)
	if err != nil {
		respondCanvasError(c, err, "add_member_failed")
		return
	}
	response.Created(c, member)
}

// UpdateMemberRole handles PUT /api/canvases/:id/members/:userId.
func (h *CanvasHandler) UpdateMemberRole(c *gin.Context) {
	canvasID, ok := parseCanvasID(c)
	if !ok {
		return
	}
	memberID, err := strconv.ParseInt(c.Param("userId"), 10, 64)
	if err != nil {
		response.BadRequest(c, "invalid member user ID")
		return
	}
	var req appCanvas.UpdateMemberRoleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	member, err := h.appSvc.UpdateMemberRole(c.Request.Context(), canvasID, identity.UserID(getUserID(c)), identity.UserID(memberID), req)
	if err != nil {
		respondCanvasError(c, err, "update_member_failed")
		return
	}
	response.OK(c, member)
}

// RemoveMember handles DELETE /api/canvases/:id/members/:userId.
func (h *CanvasHandler) RemoveMember(c *gin.Context) {
	canvasID, ok := parseCanvasID(c)
	if !ok {
		return
	}
	memberID, err := strconv.ParseInt(c.Param("userId"), 10, 64)
	if err != nil {
		response.BadRequest(c, "invalid member user ID")
		return
	}
	if err := h.appSvc.RemoveMember(c.Request.Context(), canvasID, identity.UserID(getUserID(c)), identity.UserID(memberID)); err != nil {
		respondCanvasError(c, err, "remove_member_failed")
		return
	}
	response.OK(c, gin.H{"status": "removed"})
}

// Publish handles POST /api/canvases/:id/publish.
func (h *CanvasHandler) Publish(c *gin.Context) {
	userID := getUserID(c)
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "invalid canvas ID")
		return
	}

	if err := h.appSvc.Publish(c.Request.Context(), canvas.CanvasID(id), identity.UserID(userID)); err != nil {
		response.Error(c, 400, "publish_failed", err.Error())
		return
	}
	response.OK(c, gin.H{"status": "published"})
}

// Fork handles POST /api/canvases/:id/fork.
func (h *CanvasHandler) Fork(c *gin.Context) {
	userID := getUserID(c)
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "invalid canvas ID")
		return
	}

	dto, err := h.appSvc.Fork(c.Request.Context(), canvas.CanvasID(id), identity.UserID(userID))
	if err != nil {
		if errors.Is(err, shared.ErrForbidden) {
			response.Forbidden(c, "you do not have access to fork this canvas")
		} else {
			response.Error(c, 400, "fork_failed", err.Error())
		}
		return
	}
	response.Created(c, dto)
}

// Delete handles DELETE /api/canvases/:id.
func (h *CanvasHandler) Delete(c *gin.Context) {
	userID := getUserID(c)
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "invalid canvas ID")
		return
	}

	if err := h.appSvc.Delete(c.Request.Context(), canvas.CanvasID(id), identity.UserID(userID)); err != nil {
		response.Error(c, 400, "delete_failed", err.Error())
		return
	}
	response.OK(c, gin.H{"status": "deleted"})
}

// getUserID extracts the authenticated user ID from the Gin context.
func getUserID(c *gin.Context) int64 {
	val, _ := c.Get("userID")
	return int64(val.(identity.UserID))
}

func parseCanvasID(c *gin.Context) (canvas.CanvasID, bool) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "invalid canvas ID")
		return 0, false
	}
	return canvas.CanvasID(id), true
}

func respondCanvasError(c *gin.Context, err error, fallbackCode string) {
	switch {
	case errors.Is(err, shared.ErrForbidden):
		response.Forbidden(c, err.Error())
	case errors.Is(err, shared.ErrNotFound):
		response.NotFound(c, err.Error())
	case errors.Is(err, shared.ErrConflict):
		response.Conflict(c, err.Error())
	case errors.Is(err, shared.ErrInvalidArgument):
		response.BadRequest(c, err.Error())
	default:
		response.Error(c, 500, fallbackCode, err.Error())
	}
}
