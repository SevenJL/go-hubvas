package middleware

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
)

var redisBucketScript = redis.NewScript(`
local now = redis.call('TIME')
local current = tonumber(now[1]) + tonumber(now[2]) / 1000000
local values = redis.call('HMGET', KEYS[1], 'tokens', 'updated')
local tokens = tonumber(values[1]) or tonumber(ARGV[2])
local updated = tonumber(values[2]) or current
tokens = math.min(tonumber(ARGV[2]), tokens + math.max(0, current-updated) * tonumber(ARGV[1]))
local allowed = 0
if tokens >= 1 then tokens = tokens - 1; allowed = 1 end
redis.call('HSET', KEYS[1], 'tokens', tokens, 'updated', current)
redis.call('EXPIRE', KEYS[1], tonumber(ARGV[3]))
return allowed
`)

type RateLimiter struct {
	mu        sync.Mutex
	buckets   map[string]*tokenBucket
	rate      float64
	burst     int
	lastSweep time.Time
	entryTTL  time.Duration
	redis     *redis.Client
	namespace string
}
type tokenBucket struct {
	tokens   float64
	lastTime time.Time
}

func NewRateLimiter(rate float64, burst int) *RateLimiter {
	return &RateLimiter{buckets: make(map[string]*tokenBucket), rate: rate, burst: burst, lastSweep: time.Now(), entryTTL: 30 * time.Minute, namespace: "hubvas:ratelimit"}
}
func NewRedisRateLimiter(client *redis.Client, rate float64, burst int) *RateLimiter {
	r := NewRateLimiter(rate, burst)
	r.redis = client
	return r
}
func (rl *RateLimiter) Scoped(rate float64, burst int) *RateLimiter {
	if rl != nil && rl.redis != nil {
		return NewRedisRateLimiter(rl.redis, rate, burst)
	}
	return NewRateLimiter(rate, burst)
}
func (rl *RateLimiter) Allow(key string) bool { return rl.allow(context.Background(), key) }
func (rl *RateLimiter) allow(ctx context.Context, key string) bool {
	if rl.redis != nil {
		ttl := int(max(60, float64(rl.burst)/rl.rate*2))
		result, err := redisBucketScript.Run(ctx, rl.redis, []string{rl.namespace + ":" + key}, rl.rate, rl.burst, ttl).Int()
		return err == nil && result == 1
	}
	rl.mu.Lock()
	defer rl.mu.Unlock()
	now := time.Now()
	if now.Sub(rl.lastSweep) >= time.Minute || len(rl.buckets) > 10000 {
		cutoff := now.Add(-rl.entryTTL)
		for k, b := range rl.buckets {
			if b.lastTime.Before(cutoff) {
				delete(rl.buckets, k)
			}
		}
		rl.lastSweep = now
	}
	b, ok := rl.buckets[key]
	if !ok {
		b = &tokenBucket{tokens: float64(rl.burst), lastTime: now}
		rl.buckets[key] = b
	}
	b.tokens += now.Sub(b.lastTime).Seconds() * rl.rate
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
func (rl *RateLimiter) Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		if !rl.allow(c.Request.Context(), "global:ip:"+c.ClientIP()) {
			c.Header("Retry-After", "1")
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{"code": http.StatusTooManyRequests, "message": "rate limit exceeded"})
			return
		}
		c.Next()
	}
}
func (rl *RateLimiter) KeyedMiddleware(scope string, byUser bool) gin.HandlerFunc {
	return func(c *gin.Context) {
		key := scope + ":ip:" + c.ClientIP()
		if byUser {
			if id := GetUserID(c); id != 0 {
				key = fmt.Sprintf("%s:user:%d", scope, id)
			}
		}
		if !rl.allow(c.Request.Context(), key) {
			c.Header("Retry-After", "1")
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{"code": http.StatusTooManyRequests, "message": "rate limit exceeded"})
			return
		}
		c.Next()
	}
}
