package middleware

import (
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

// RateLimiter implements a simple token-bucket rate limiter per key.
// For production, replace with a Redis-backed implementation.
type RateLimiter struct {
	mu        sync.Mutex
	buckets   map[string]*tokenBucket
	rate      float64 // tokens per second
	burst     int     // max burst size
	lastSweep time.Time
	entryTTL  time.Duration
}

type tokenBucket struct {
	tokens   float64
	lastTime time.Time
}

// NewRateLimiter creates a per-key rate limiter.
func NewRateLimiter(rate float64, burst int) *RateLimiter {
	return &RateLimiter{
		buckets:   make(map[string]*tokenBucket),
		rate:      rate,
		burst:     burst,
		lastSweep: time.Now(),
		entryTTL:  30 * time.Minute,
	}
}

// Allow checks if a request identified by key is allowed.
func (rl *RateLimiter) Allow(key string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	// Anonymous/IP keys are unbounded input. Periodically evict idle entries so
	// scanners cannot grow the process map forever.
	if now.Sub(rl.lastSweep) >= time.Minute || len(rl.buckets) > 10_000 {
		cutoff := now.Add(-rl.entryTTL)
		for existingKey, bucket := range rl.buckets {
			if bucket.lastTime.Before(cutoff) {
				delete(rl.buckets, existingKey)
			}
		}
		rl.lastSweep = now
	}

	b, ok := rl.buckets[key]
	if !ok {
		b = &tokenBucket{tokens: float64(rl.burst), lastTime: time.Now()}
		rl.buckets[key] = b
	}

	elapsed := now.Sub(b.lastTime).Seconds()
	b.tokens += elapsed * rl.rate
	if b.tokens > float64(rl.burst) {
		b.tokens = float64(rl.burst)
	}
	b.lastTime = now

	if b.tokens >= 1 {
		b.tokens--
		return true
	}
	return false
}

// Middleware returns a Gin middleware that rate-limits by IP.
func (rl *RateLimiter) Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		key := c.ClientIP()
		if !rl.Allow(key) {
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
				"code":    http.StatusTooManyRequests,
				"message": "rate limit exceeded",
			})
			return
		}
		c.Next()
	}
}

// KeyedMiddleware creates an endpoint-specific limiter key. Protected routes can
// key by authenticated user; anonymous routes fall back to IP.
func (rl *RateLimiter) KeyedMiddleware(scope string, byUser bool) gin.HandlerFunc {
	return func(c *gin.Context) {
		key := scope + ":ip:" + c.ClientIP()
		if byUser {
			if id := GetUserID(c); id != 0 {
				key = fmt.Sprintf("%s:user:%d", scope, id)
			}
		}
		if !rl.Allow(key) {
			c.Header("Retry-After", "1")
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{"code": http.StatusTooManyRequests, "message": "rate limit exceeded"})
			return
		}
		c.Next()
	}
}
