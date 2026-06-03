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
// Body: { "data": <tldraw JSON>, "thumbnail": "<base64 data URL>" }
func (h *SnapshotHandler) Save(c *gin.Context) {
	canvasID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "invalid canvas ID")
		return
	}

	body, err := io.ReadAll(c.Request.Body)
	if err != nil || len(body) == 0 {
		response.BadRequest(c, "empty body")
		return
	}

	var req struct {
		Data      json.RawMessage `json:"data"`
		Thumbnail string          `json:"thumbnail"`
	}
	if err := json.Unmarshal(body, &req); err != nil {
		response.BadRequest(c, "invalid JSON: "+err.Error())
		return
	}
	if len(req.Data) == 0 {
		response.BadRequest(c, "missing data field")
		return
	}

	userID, _ := c.Get("userID")
	if err := h.appSvc.Save(c.Request.Context(), canvasDomain.CanvasID(canvasID), userID.(identity.UserID), req.Data, req.Thumbnail); err != nil {
		response.Error(c, 403, "save_failed", err.Error())
		return
	}
	response.OK(c, gin.H{"saved": true})
}

// Load handles GET /api/canvases/:id/snapshot.
// Returns { "data": <tldraw JSON>, "thumbnail": "<base64 data URL or empty>" }
func (h *SnapshotHandler) Load(c *gin.Context) {
	canvasID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "invalid canvas ID")
		return
	}

	userID, _ := c.Get("userID")
	data, thumbnail, err := h.appSvc.Load(c.Request.Context(), canvasDomain.CanvasID(canvasID), userID.(identity.UserID))
	if err != nil {
		response.Error(c, 403, "load_failed", err.Error())
		return
	}
	if data == nil {
		response.OK(c, nil)
		return
	}

	var snapshot interface{}
	json.Unmarshal(data, &snapshot)
	response.OK(c, gin.H{
		"data":      snapshot,
		"thumbnail": thumbnail,
	})
}
