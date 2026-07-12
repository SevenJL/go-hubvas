package handler

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
)

type DependencyCheck struct {
	Name  string
	Check func(context.Context) error
}
type HealthHandler struct{ checks []DependencyCheck }

func NewHealthHandler(db *pgxpool.Pool, extra ...DependencyCheck) *HealthHandler {
	checks := []DependencyCheck{{Name: "database", Check: db.Ping}}
	checks = append(checks, extra...)
	return &HealthHandler{checks: checks}
}
func (h *HealthHandler) Health(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "ok", "time": time.Now().UTC().Format(time.RFC3339)})
}
func (h *HealthHandler) Ready(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 2*time.Second)
	defer cancel()
	statuses := gin.H{}
	ready := true
	for _, dep := range h.checks {
		if dep.Check == nil {
			continue
		}
		if err := dep.Check(ctx); err != nil {
			statuses[dep.Name] = "unavailable"
			ready = false
		} else {
			statuses[dep.Name] = "ok"
		}
	}
	if !ready {
		c.JSON(http.StatusServiceUnavailable, gin.H{"status": "not ready", "dependencies": statuses})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "ready", "dependencies": statuses})
}
