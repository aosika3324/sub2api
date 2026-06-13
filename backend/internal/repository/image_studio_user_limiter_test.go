//go:build unit

package repository

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
)

func newStudioUserLimiterTest(t *testing.T) (*studioUserLimiter, *miniredis.Miniredis) {
	t.Helper()
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })
	return &studioUserLimiter{rdb: rdb}, mr
}

func TestStudioUserLimiter_AcquireUpToCapThenReject(t *testing.T) {
	l, _ := newStudioUserLimiterTest(t)
	ctx := context.Background()
	const userID = int64(42)

	rel1, ok, err := l.Acquire(ctx, userID, 2, time.Minute)
	require.NoError(t, err)
	require.True(t, ok, "first slot acquired")

	rel2, ok, err := l.Acquire(ctx, userID, 2, time.Minute)
	require.NoError(t, err)
	require.True(t, ok, "second slot acquired")

	_, ok, err = l.Acquire(ctx, userID, 2, time.Minute)
	require.NoError(t, err)
	require.False(t, ok, "third slot rejected at cap")

	// Releasing one frees a slot for the next acquire.
	rel1()
	rel3, ok, err := l.Acquire(ctx, userID, 2, time.Minute)
	require.NoError(t, err)
	require.True(t, ok, "slot available after release")

	rel2()
	rel3()
}

func TestStudioUserLimiter_PerUserIsolation(t *testing.T) {
	l, _ := newStudioUserLimiterTest(t)
	ctx := context.Background()

	_, ok, err := l.Acquire(ctx, 1, 1, time.Minute)
	require.NoError(t, err)
	require.True(t, ok)

	// A different user is unaffected by user 1 being at cap.
	_, ok, err = l.Acquire(ctx, 2, 1, time.Minute)
	require.NoError(t, err)
	require.True(t, ok, "other user has an independent counter")

	// User 1 is still at cap.
	_, ok, err = l.Acquire(ctx, 1, 1, time.Minute)
	require.NoError(t, err)
	require.False(t, ok)
}

func TestStudioUserLimiter_ReleaseIsIdempotent(t *testing.T) {
	l, _ := newStudioUserLimiterTest(t)
	ctx := context.Background()
	const userID = int64(7)

	rel, ok, err := l.Acquire(ctx, userID, 1, time.Minute)
	require.NoError(t, err)
	require.True(t, ok)

	rel()
	rel() // second call must not over-decrement

	// Counter is back to zero: one slot is acquirable, a second is not.
	_, ok, err = l.Acquire(ctx, userID, 1, time.Minute)
	require.NoError(t, err)
	require.True(t, ok, "released slot is reusable")

	_, ok, err = l.Acquire(ctx, userID, 1, time.Minute)
	require.NoError(t, err)
	require.False(t, ok, "double-release did not corrupt the counter")
}

func TestStudioUserLimiter_MaxZeroDisablesGate(t *testing.T) {
	l, _ := newStudioUserLimiterTest(t)
	ctx := context.Background()

	for i := 0; i < 5; i++ {
		_, ok, err := l.Acquire(ctx, 1, 0, time.Minute)
		require.NoError(t, err)
		require.True(t, ok, "max<=0 always acquires")
	}
}

func TestStudioUserLimiter_TTLExpiry(t *testing.T) {
	l, mr := newStudioUserLimiterTest(t)
	ctx := context.Background()
	const userID = int64(99)

	_, ok, err := l.Acquire(ctx, userID, 1, 30*time.Second)
	require.NoError(t, err)
	require.True(t, ok)

	// At cap before expiry.
	_, ok, _ = l.Acquire(ctx, userID, 1, 30*time.Second)
	require.False(t, ok)

	// After the slot's TTL elapses, a crashed-process slot self-heals.
	mr.FastForward(31 * time.Second)
	_, ok, err = l.Acquire(ctx, userID, 1, 30*time.Second)
	require.NoError(t, err)
	require.True(t, ok, "slot self-heals after TTL")
}
