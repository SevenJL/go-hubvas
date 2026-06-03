package handler

import (
	"encoding/json"
	"io"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/hubvas/internal/infrastructure/persistence/postgres"
	"github.com/hubvas/internal/interfaces/http/response"
)

// SnapshotHandler handles saving and loading tldraw store snapshots.
type SnapshotHandler struct {
	store *postgres.SnapshotStore
}

// NewSnapshotHandler creates a SnapshotHandler.
func NewSnapshotHandler(store *postgres.SnapshotStore) *SnapshotHandler {
	return &SnapshotHandler{store: store}
}

// Save handles PUT /api/canvases/:id/snapshot.
// The request body is the tldraw store JSON snapshot.
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

	// Validate it's valid JSON.
	if !json.Valid(body) {
		response.BadRequest(c, "snapshot must be valid JSON")
		return
	}

	if err := h.store.SaveSnapshot(c.Request.Context(), canvasID, body); err != nil {
		response.InternalError(c, "failed to save snapshot: "+err.Error())
		return
	}

	response.OK(c, gin.H{"saved": true})
}

// Load handles GET /api/canvases/:id/snapshot.
// Returns the stored snapshot, or null if none exists.
func (h *SnapshotHandler) Load(c *gin.Context) {
	canvasID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "invalid canvas ID")
		return
	}

	data, err := h.store.LoadSnapshot(c.Request.Context(), canvasID)
	if err != nil {
		response.InternalError(c, "failed to load snapshot: "+err.Error())
		return
	}

	if data == nil {
		response.OK(c, nil)
		return
	}

	// Return the raw JSON snapshot.
	var snapshot interface{}
	if err := json.Unmarshal(data, &snapshot); err != nil {
		response.InternalError(c, "failed to parse snapshot")
		return
	}
	response.OK(c, snapshot)
}
