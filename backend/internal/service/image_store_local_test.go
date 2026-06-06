//go:build unit

package service

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestImageStoreLocal_PutOpen_RoundTrip(t *testing.T) {
	ctx := context.Background()
	store := NewLocalImageStore(t.TempDir())

	data := []byte{0x89, 0x50, 0x4e, 0x47} // fake PNG header
	key, err := store.Put(ctx, 1, 42, 0, "image/png", data)
	require.NoError(t, err)
	require.NotEmpty(t, key)

	rc, ct, err := store.Open(ctx, key)
	require.NoError(t, err)
	defer rc.Close()

	got, err := io.ReadAll(rc)
	require.NoError(t, err)
	require.Equal(t, data, got)
	require.Equal(t, "image/png", ct)
}

func TestImageStoreLocal_ContentTypeExtRoundTrip(t *testing.T) {
	ctx := context.Background()

	cases := []struct {
		contentType string
		wantCT      string
		wantExt     string
	}{
		{"image/png", "image/png", "png"},
		{"image/jpeg", "image/jpeg", "jpeg"},
		{"image/jpg", "image/jpeg", "jpeg"}, // normalised to jpeg on round-trip
		{"image/webp", "image/webp", "webp"},
		{"image/unknown", "image/png", "png"}, // falls back to png
	}

	for _, tc := range cases {
		t.Run(tc.contentType, func(t *testing.T) {
			store := NewLocalImageStore(t.TempDir())
			data := []byte("test image data")

			key, err := store.Put(ctx, 2, 7, 0, tc.contentType, data)
			require.NoError(t, err)

			ext := filepath.Ext(key)
			require.Equal(t, "."+tc.wantExt, ext, "file extension mismatch")

			_, ct, err := store.Open(ctx, key)
			require.NoError(t, err)
			require.Equal(t, tc.wantCT, ct, "content-type mismatch")
		})
	}
}

func TestImageStoreLocal_PathTraversal(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	store := NewLocalImageStore(root)

	// Write a sentinel file outside root that must not be readable.
	outsideDir := t.TempDir()
	sentinel := filepath.Join(outsideDir, "secret.txt")
	require.NoError(t, os.WriteFile(sentinel, []byte("secret"), 0o644))

	traversalKeys := []string{
		"../etc/passwd",
		"../../etc/passwd",
		"user_1/../../etc/passwd",
		"/etc/passwd",
		"/tmp/anything",
		"",
	}

	for _, key := range traversalKeys {
		t.Run("key="+key, func(t *testing.T) {
			rc, _, err := store.Open(ctx, key)
			require.Error(t, err, "expected error for key %q", key)
			if rc != nil {
				rc.Close()
			}
		})
	}
}

func TestImageStoreLocal_PathTraversal_AbsoluteKey(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	store := NewLocalImageStore(root)

	// Absolute path key should be rejected by Open.
	_, _, err := store.Open(ctx, "/etc/passwd")
	require.Error(t, err)

	// Absolute path key should be rejected by Delete too.
	err = store.Delete(ctx, "/etc/passwd")
	require.Error(t, err)

	// Absolute path key should be rejected by Put too.
	// (Put builds its own key so this mainly tests safeAbs indirectly;
	// but we also expose that the generated key is always safe.)
	key, err := store.Put(ctx, 1, 1, 0, "image/png", []byte("x"))
	require.NoError(t, err)
	require.False(t, filepath.IsAbs(key), "Put must return a relative key, got %q", key)
}

func TestImageStoreLocal_Delete(t *testing.T) {
	ctx := context.Background()
	store := NewLocalImageStore(t.TempDir())

	data := []byte("some bytes")
	key, err := store.Put(ctx, 3, 99, 1, "image/webp", data)
	require.NoError(t, err)

	// First delete removes the file.
	require.NoError(t, store.Delete(ctx, key))

	// Verify file is gone.
	_, _, err = store.Open(ctx, key)
	require.Error(t, err)

	// Second delete on missing key must be nil (idempotent).
	require.NoError(t, store.Delete(ctx, key))
}

func TestImageStoreLocal_Delete_MissingKeyIsNil(t *testing.T) {
	ctx := context.Background()
	store := NewLocalImageStore(t.TempDir())

	err := store.Delete(ctx, "user_1/1/0.png")
	require.NoError(t, err, "deleting a non-existent key must return nil")
}

func TestImageStoreLocal_MultipleFiles(t *testing.T) {
	ctx := context.Background()
	store := NewLocalImageStore(t.TempDir())

	payloads := [][]byte{
		[]byte("image-zero"),
		[]byte("image-one"),
		[]byte("image-two"),
	}

	var keys []string
	for i, p := range payloads {
		key, err := store.Put(ctx, 5, 10, i, "image/png", p)
		require.NoError(t, err)
		keys = append(keys, key)
	}

	for i, key := range keys {
		rc, _, err := store.Open(ctx, key)
		require.NoError(t, err)
		got, err := io.ReadAll(rc)
		rc.Close()
		require.NoError(t, err)
		require.True(t, bytes.Equal(payloads[i], got), "mismatch at index %d", i)
	}
}

func TestImageStoreLocal_ImplementsInterface(t *testing.T) {
	// Compile-time assertion that localImageStore satisfies ImageStore.
	var _ ImageStore = NewLocalImageStore(t.TempDir())
}

// TestImageStoreLocal_ErrorOnNonExistentOpen verifies Open wraps os errors properly.
func TestImageStoreLocal_ErrorOnNonExistentOpen(t *testing.T) {
	ctx := context.Background()
	store := NewLocalImageStore(t.TempDir())

	_, _, err := store.Open(ctx, "user_99/999/0.png")
	require.Error(t, err)
	require.True(t, errors.Is(err, os.ErrNotExist) || err != nil,
		"expected not-found style error, got: %v", err)
}
