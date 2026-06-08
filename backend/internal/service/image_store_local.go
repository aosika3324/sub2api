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
// A relative rootDir is normalised to absolute so the safeAbs containment check
// is stable regardless of later cwd changes.
func NewLocalImageStore(rootDir string) *localImageStore {
	if abs, err := filepath.Abs(rootDir); err == nil {
		rootDir = abs
	}
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
	case "image/gif":
		return "gif"
	case "image/avif":
		return "avif"
	case "image/svg+xml":
		return "svg"
	case "image/bmp":
		return "bmp"
	case "image/tiff":
		return "tiff"
	case "image/heic":
		return "heic"
	case "image/heif":
		return "heif"
	default:
		// Covers "image/png" and any unknown type.
		return "png"
	}
}

// contentTypeForExt maps file extensions back to MIME content-types. It must stay
// the exact inverse of extForContentType so a stored image round-trips losslessly.
func contentTypeForExt(ext string) string {
	switch strings.ToLower(ext) {
	case "jpeg", "jpg":
		return "image/jpeg"
	case "webp":
		return "image/webp"
	case "gif":
		return "image/gif"
	case "avif":
		return "image/avif"
	case "svg":
		return "image/svg+xml"
	case "bmp":
		return "image/bmp"
	case "tiff":
		return "image/tiff"
	case "heic":
		return "image/heic"
	case "heif":
		return "image/heif"
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
	// Ensure the resolved path is strictly *inside* rootDir.
	// The trailing separator avoids a prefix collision where rootDir="/tmp/root"
	// would incorrectly accept "/tmp/root2/...". The check is unconditional: a key
	// that cleans to rootDir itself (e.g. "." or "a/..") is rejected, otherwise
	// Delete(".") on an empty store could remove the root directory.
	root := filepath.Clean(s.rootDir)
	if !strings.HasPrefix(abs, root+string(filepath.Separator)) {
		return "", fmt.Errorf("image store: key %q escapes root directory", key)
	}
	return abs, nil
}

// Put stores produced image data and returns the relative storage key.
// Key format: user_{userID}/{genID}/{idx}.{ext}
func (s *localImageStore) Put(_ context.Context, userID, genID int64, idx int, contentType string, data []byte) (string, error) {
	key := fmt.Sprintf("user_%d/%d/%d.%s", userID, genID, idx, extForContentType(contentType))
	return s.writeKey(key, data)
}

// PutInput stores a user-provided (input/reference) image and returns the
// relative storage key. Key format: user_{userID}/{genID}/input/{idx}.{ext}.
// The "input/" segment keeps reference images from colliding with output keys
// while staying within rootDir (safeAbs containment still applies).
func (s *localImageStore) PutInput(_ context.Context, userID, genID int64, idx int, contentType string, data []byte) (string, error) {
	key := fmt.Sprintf("user_%d/%d/input/%d.%s", userID, genID, idx, extForContentType(contentType))
	return s.writeKey(key, data)
}

// writeKey resolves key under rootDir (with traversal protection), creates the
// parent directory, and writes data. Shared by Put and PutInput.
func (s *localImageStore) writeKey(key string, data []byte) (string, error) {
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
