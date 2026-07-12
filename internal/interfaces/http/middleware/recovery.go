package middleware

import (
	"log/slog"
	"net/http"
	"runtime/debug"

	"github.com/gin-gonic/gin"
)

// Recovery returns a middleware that recovers from panics without leaking
// internals and records the request ID needed to correlate the failure.
func Recovery() gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if err := recover(); err != nil {
				slog.ErrorContext(c.Request.Context(), "http panic recovered", "request_id", requestIDValue(c), "panic", err, "stack", string(debug.Stack()))
				c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
					"code":       "internal_error",
					"message":    "internal server error",
					"request_id": requestIDValue(c),
				})
			}
		}()
		c.Next()
	}
}
