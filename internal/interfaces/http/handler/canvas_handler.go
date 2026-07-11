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
