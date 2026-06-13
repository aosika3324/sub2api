package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"

	"github.com/redis/go-redis/v9"
)

// studioUserLimiterKeyPrefix namespaces the per-user in-flight counter.
const studioUserLimiterKeyPrefix = "studio:inflight:"

var (
	// studioAcquireScript atomically reserves one per-user in-flight slot.
	// KEYS[1] = per-user counter key
	// ARGV[1] = max concurrent
	// ARGV[2] = TTL seconds (bounds the slot so a crashed process self-heals)
	// Returns 1 when acquired, 0 when the user is already at the cap.
	studioAcquireScript = redis.NewScript(`
		local current = redis.call('GET', KEYS[1])
		if current == false then
			current = 0
		else
			current = tonumber(current)
		end
		if current >= tonumber(ARGV[1]) then
			return 0
		end
		redis.call('INCR', KEYS[1])
		redis.call('EXPIRE', KEYS[1], ARGV[2])
		return 1
	`)

	// studioReleaseScript frees one slot without dropping below zero, and clears
	// the key once it reaches zero to avoid leaking idle keys.
	// KEYS[1] = per-user counter key
	studioReleaseScript = redis.NewScript(`
		local current = redis.call('GET', KEYS[1])
		if current == false then
			return 0
		end
		current = tonumber(current)
		if current <= 1 then
			redis.call('DEL', KEYS[1])
			return 0
		end
		redis.call('DECR', KEYS[1])
		return current - 1
	`)
)

type studioUserLimiter struct {
	rdb *redis.Client
}

// NewStudioUserLimiter creates a Redis-backed per-user in-flight limiter for the
// image studio. Returns nil when rdb is nil so callers can transparently skip
// the gate (e.g. Redis unavailable / unit tests).
func NewStudioUserLimiter(rdb *redis.Client) service.StudioUserLimiter {
	if rdb == nil {
		return nil
	}
	return &studioUserLimiter{rdb: rdb}
}

func studioUserLimiterKey(userID int64) string {
	return fmt.Sprintf("%s%d", studioUserLimiterKeyPrefix, userID)
}

// Acquire reserves one in-flight slot for userID. It returns ok=false (release
// nil) when the user already holds max slots. The returned release decrements
// the counter exactly once; ttl bounds the slot's lifetime so a crashed process
// self-heals. max<=0 disables the gate (always acquires, no-op release).
func (l *studioUserLimiter) Acquire(ctx context.Context, userID int64, max int, ttl time.Duration) (func(), bool, error) {
	if l == nil || l.rdb == nil || max <= 0 {
		return func() {}, true, nil
	}
	ttlSeconds := int(ttl.Seconds())
	if ttlSeconds <= 0 {
		ttlSeconds = int((15 * time.Minute).Seconds())
	}

	key := studioUserLimiterKey(userID)
	res, err := studioAcquireScript.Run(ctx, l.rdb, []string{key}, max, ttlSeconds).Int()
	if err != nil {
		return nil, false, fmt.Errorf("studio user limiter acquire: %w", err)
	}
	if res == 0 {
		return nil, false, nil
	}

	released := false
	release := func() {
		if released {
			return
		}
		released = true
		// Release uses a fresh, short-lived context: the original request ctx is
		// often already canceled by the time the background job releases.
		relCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_, _ = studioReleaseScript.Run(relCtx, l.rdb, []string{key}).Result()
	}
	return release, true, nil
}
