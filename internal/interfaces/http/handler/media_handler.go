package handler

import (
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	appmedia "github.com/hubvas/internal/application/media"
	"github.com/hubvas/internal/interfaces/http/middleware"
	"github.com/hubvas/internal/interfaces/http/response"
	"strconv"
)

type MediaHandler struct{ svc *appmedia.Service }

func NewMediaHandler(s *appmedia.Service) *MediaHandler { return &MediaHandler{svc: s} }
func (h *MediaHandler) Presign(c *gin.Context) {
	var in appmedia.PresignRequest
	if err := c.ShouldBindJSON(&in); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	v, err := h.svc.Presign(c.Request.Context(), middleware.GetUserID(c), in)
	if err != nil {
		socialError(c, err)
		return
	}
	response.Created(c, v)
}
func (h *MediaHandler) Complete(c *gin.Context) {
	var in appmedia.CompleteRequest
	if err := c.ShouldBindJSON(&in); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	v, err := h.svc.Complete(c.Request.Context(), middleware.GetUserID(c), in)
	if err != nil {
		socialError(c, err)
		return
	}
	response.OK(c, v)
}
func (h *MediaHandler) Upload(c *gin.Context) {
	// Limit the entire multipart request before Gin parses it. The extra MiB
	// allows multipart headers and crop fields while keeping the file itself
	// subject to the stricter service-level limit.
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, h.svc.MaxUploadBytes()+(1<<20))
	file, err := c.FormFile("file")
	if err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			response.Error(c, http.StatusRequestEntityTooLarge, "payload_too_large", "avatar upload request is too large")
		} else {
			response.BadRequest(c, "avatar file is required")
		}
		return
	}
	crop, err := parseCrop(c)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	v, err := h.svc.Multipart(c.Request.Context(), middleware.GetUserID(c), file, crop)
	if err != nil {
		socialError(c, err)
		return
	}
	response.OK(c, v)
}
func parseCrop(c *gin.Context) (*appmedia.Crop, error) {
	names := []string{"crop_x", "crop_y", "crop_width", "crop_height"}
	v := make([]float64, 4)
	present := 0
	for i, name := range names {
		raw := strings.TrimSpace(c.PostForm(name))
		if raw == "" {
			continue
		}
		present++
		value, err := strconv.ParseFloat(raw, 64)
		if err != nil {
			return nil, fmt.Errorf("%s must be a number", name)
		}
		v[i] = value
	}
	if present == 0 {
		return nil, nil
	}
	if present != len(names) {
		return nil, errors.New("all crop fields are required when specifying a crop")
	}
	return &appmedia.Crop{X: v[0], Y: v[1], Width: v[2], Height: v[3]}, nil
}
func (h *MediaHandler) Remove(c *gin.Context) {
	if err := h.svc.Remove(c.Request.Context(), middleware.GetUserID(c)); err != nil {
		socialError(c, err)
		return
	}
	response.OK(c, gin.H{"avatar_url": ""})
}
