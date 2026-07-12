package middleware

import (
	"context"
	"log/slog"
	"net/http"
	"regexp"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/hubvas/internal/domain/identity"
	"github.com/prometheus/client_golang/prometheus"
)

const RequestIDKey = "requestID"

type requestIDContextKey struct{}

var requestIDPattern = regexp.MustCompile(`^[A-Za-z0-9._:-]{8,128}$`)

var (
	httpRequests = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: "hubvas", Subsystem: "http", Name: "requests_total", Help: "Completed HTTP requests.",
	}, []string{"method", "route", "status"})
	httpDuration = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: "hubvas", Subsystem: "http", Name: "request_duration_seconds", Help: "HTTP request duration.", Buckets: prometheus.DefBuckets,
	}, []string{"method", "route", "status"})
	httpInflight = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: "hubvas", Subsystem: "http", Name: "inflight_requests", Help: "HTTP requests currently being served.",
	})
)

func init() {
	prometheus.MustRegister(httpRequests, httpDuration, httpInflight)
}

func RequestID() gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.GetHeader("X-Request-ID")
		if !requestIDPattern.MatchString(id) {
			id = uuid.NewString()
		}
		c.Set(RequestIDKey, id)
		c.Header("X-Request-ID", id)
		c.Request = c.Request.WithContext(context.WithValue(c.Request.Context(), requestIDContextKey{}, id))
		c.Next()
	}
}

func RequestIDFromContext(ctx context.Context) string {
	id, _ := ctx.Value(requestIDContextKey{}).(string)
	return id
}

func Metrics() gin.HandlerFunc {
	return func(c *gin.Context) {
		started := time.Now()
		httpInflight.Inc()
		defer httpInflight.Dec()
		c.Next()
		route := c.FullPath()
		if route == "" {
			route = "unmatched"
		}
		status := strconv.Itoa(c.Writer.Status())
		httpRequests.WithLabelValues(c.Request.Method, route, status).Inc()
		httpDuration.WithLabelValues(c.Request.Method, route, status).Observe(time.Since(started).Seconds())
	}
}

func AccessLog(logger *slog.Logger) gin.HandlerFunc {
	if logger == nil {
		logger = slog.Default()
	}
	return func(c *gin.Context) {
		started := time.Now()
		c.Next()
		attrs := []any{
			"request_id", requestIDValue(c),
			"method", c.Request.Method,
			"route", c.FullPath(),
			"status", c.Writer.Status(),
			"latency_ms", time.Since(started).Milliseconds(),
			"client_ip", c.ClientIP(),
			"response_bytes", c.Writer.Size(),
		}
		if value, exists := c.Get("userID"); exists {
			if id, ok := value.(identity.UserID); ok {
				attrs = append(attrs, "user_id", int64(id))
			}
		}
		level := slog.LevelInfo
		if c.Writer.Status() >= http.StatusInternalServerError {
			level = slog.LevelError
		} else if c.Writer.Status() >= http.StatusBadRequest {
			level = slog.LevelWarn
		}
		logger.Log(c.Request.Context(), level, "http request", attrs...)
	}
}

func requestIDValue(c *gin.Context) string {
	value, _ := c.Get(RequestIDKey)
	id, _ := value.(string)
	return id
}
