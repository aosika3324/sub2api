//go:build unit

package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestImageConcurrencyLimiter_AcquireReleaseAcquire(t *testing.T) {
	l := &ImageConcurrencyLimiter{}

	// First acquire with limit=1 should succeed.
	release, acquired := l.Acquire(context.Background(), true, 1, false, 0, 0)
	require.True(t, acquired, "first Acquire should succeed")
	require.NotNil(t, release, "first Acquire should return a non-nil release func")

	// Second acquire while first is still held should fail (no wait).
	_, acquired2 := l.Acquire(context.Background(), true, 1, false, 0, 0)
	require.False(t, acquired2, "second Acquire should fail while limit is reached")

	// After releasing, a third acquire should succeed again.
	release()
	release3, acquired3 := l.Acquire(context.Background(), true, 1, false, 0, 0)
	require.True(t, acquired3, "third Acquire after release should succeed")
	require.NotNil(t, release3)
	release3()
}
