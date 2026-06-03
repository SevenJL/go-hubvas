package handler

import (
	"encoding/json"
	"io"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/hubvas/internal/application/canvas"
	canvasDomain "github.com/hubvas/internal/domain/canvas"
	"github.com/hubvas/internal/domain/identity"
	"github.com/hubvas/internal/interfaces/http/response"
)

// SnapshotHandler handles saving and loading canvas visual snapshots.
type SnapshotHandler struct {
	appSvc *canvas.SnapshotApplicationService
}

// NewSnapshotHandler creates a SnapshotHandler.
func NewSnapshotHandler(appSvc *canvas.SnapshotApplicationService) *SnapshotHandler {
	return &SnapshotHandler{appSvc: appSvc}
}

// Save handles PUT /api/canvases/:id/snapshot.
func (h *SnapshotHandler) Save(c *gin.Context) {
	canvasID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "invalid canvas ID")
		return
	}

	body, err := io.ReadAll(c.Request.Body)
	if err != nil || len(body) == 0 {
		response.BadRequest(c, "empty snapshot body")
		return
	}
	if !json.Valid(body) {
		response.BadRequest(c, "snapshot must be valid JSON")
		return
	}

	userID, _ := c.Get("userID")
	if err := h.appSvc.Save(c.Request.Context(), canvasDomain.CanvasID(canvasID), userID.(identity.UserID), body); err != nil {
		response.Error(c, 403, "save_failed", err.Error())
		return
	}
	response.OK(c, gin.H{"saved": true})
}

// Load handles GET /api/canvases/:id/snapshot.
func (h *SnapshotHandler) Load(c *gin.Context) {
	canvasID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "invalid canvas ID")
		return
	}

	userID, _ := c.Get("userID")
	data, err := h.appSvc.Load(c.Request.Context(), canvasDomain.CanvasID(canvasID), userID.(identity.UserID))
	if err != nil {
		response.Error(c, 403, "load_failed", err.Error())
		return
	}
	if data == nil {
		response.OK(c, nil)
		return
	}

	var snapshot interface{}
	if err := json.Unmarshal(data, &snapshot); err != nil {
		response.InternalError(c, "failed to parse snapshot")
		return
	}
	response.OK(c, snapshot)
}
