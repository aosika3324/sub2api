// Package service provides business logic and domain services for the application.
package service

import (
	"context"
	"io"
)

// ImageStore persists generated image bytes and serves them back.
type ImageStore interface {
	// Put stores one image and returns an opaque storage key.
	Put(ctx context.Context, userID, genID int64, idx int, contentType string, data []byte) (key string, err error)
	// Open returns a reader for a stored image plus its content type.
	Open(ctx context.Context, key string) (io.ReadCloser, string, error)
	// Delete removes a stored image (idempotent: missing key is not an error).
	Delete(ctx context.Context, key string) error
}
