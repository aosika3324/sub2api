package service

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// localImageStore is a disk-backed ImageStore implementation.
// rootDir is injected by the caller; this struct never calls setup.GetDataDir.
type localImageStore struct {
	rootDir string
}

// NewLocalImageStore constructs a localImageStore rooted at rootDir.
// rootDir should be an absolute path; the directory is created lazily on first Put.
func NewLocalImageStore(rootDir string) *localImageStore {
	return &localImageStore{rootDir: rootDir}
}

// extForContentType maps MIME content-types to file extensions.
// Content-type is inferred from extension on Open, so the mapping must be 1-to-1.
func extForContentType(contentType string) string {
	switch strings.ToLower(strings.TrimSpace(contentType)) {
	case "image/jpeg", "image/jpg":
		return "jpeg"
	case "image/webp":
		return "webp"
	default:
		// Covers "image/png" and any unknown type.
		return "png"
	}
}

// contentTypeForExt maps file extensions back to MIME content-types.
func contentTypeForExt(ext string) string {
	switch strings.ToLower(ext) {
	case "jpeg", "jpg":
		return "image/jpeg"
	case "webp":
		return "image/webp"
	default:
		return "image/png"
	}
}

// safeAbs resolves key relative to rootDir and returns the cleaned absolute
// path. It returns an error if the result escapes rootDir (path traversal).
func (s *localImageStore) safeAbs(key string) (string, error) {
	if key == "" {
		return "", errors.New("image store: empty key")
	}
	// Reject keys that are already absolute paths.
	if filepath.IsAbs(key) {
		return "", fmt.Errorf("image store: key must be relative, got %q", key)
	}

	abs := filepath.Clean(filepath.Join(s.rootDir, key))
	// Ensure the resolved path is inside rootDir.
	// We compare against rootDir with a trailing separator to avoid a prefix
	// collision where rootDir="/tmp/root" would incorrectly accept "/tmp/root2/...".
	root := filepath.Clean(s.rootDir)
	if abs != root && !strings.HasPrefix(abs, root+string(filepath.Separator)) {
		return "", fmt.Errorf("image store: key %q escapes root directory", key)
	}
	return abs, nil
}

// Put stores image data and returns the relative storage key.
// Key format: user_{userID}/{genID}/{idx}.{ext}
func (s *localImageStore) Put(_ context.Context, userID, genID int64, idx int, contentType string, data []byte) (string, error) {
	ext := extForContentType(contentType)
	key := fmt.Sprintf("user_%d/%d/%d.%s", userID, genID, idx, ext)

	abs, err := s.safeAbs(key)
	if err != nil {
		return "", err
	}

	if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
		return "", fmt.Errorf("image store: mkdir: %w", err)
	}
	if err := os.WriteFile(abs, data, 0o644); err != nil {
		return "", fmt.Errorf("image store: write: %w", err)
	}
	return key, nil
}

// Open returns a reader and the content-type for the stored image.
// Content-type is inferred from the file extension in the key.
func (s *localImageStore) Open(_ context.Context, key string) (io.ReadCloser, string, error) {
	abs, err := s.safeAbs(key)
	if err != nil {
		return nil, "", err
	}

	f, err := os.Open(abs)
	if err != nil {
		return nil, "", fmt.Errorf("image store: open %q: %w", key, err)
	}

	ext := strings.TrimPrefix(filepath.Ext(abs), ".")
	ct := contentTypeForExt(ext)
	return f, ct, nil
}

// Delete removes the stored image. Missing keys are silently ignored (idempotent).
func (s *localImageStore) Delete(_ context.Context, key string) error {
	abs, err := s.safeAbs(key)
	if err != nil {
		return err
	}

	if err := os.Remove(abs); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("image store: delete %q: %w", key, err)
	}
	return nil
}
