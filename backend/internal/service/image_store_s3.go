package service

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"path"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	v4 "github.com/aws/aws-sdk-go-v2/aws/signer/v4"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	smithy "github.com/aws/smithy-go"
)

// s3ImageStore is an S3-compatible object-storage ImageStore implementation,
// enabling multi-instance deployments where any instance can serve any image.
// It shares the exact key scheme + content-type mapping with localImageStore
// (via imageStudioOutputKey/imageStudioInputKey/extForContentType) so a
// generation's keys are identical regardless of backend.
type s3ImageStore struct {
	client *s3.Client
	bucket string
	prefix string
}

// NewS3ImageStore builds an S3-backed ImageStore from the studio S3 config.
func NewS3ImageStore(ctx context.Context, cfg S3ImageStoreConfig) (ImageStore, error) {
	if strings.TrimSpace(cfg.Bucket) == "" {
		return nil, errors.New("image store: s3 bucket is required")
	}
	region := cfg.Region
	if region == "" {
		region = "auto" // Cloudflare R2 default
	}

	awsCfg, err := awsconfig.LoadDefaultConfig(ctx,
		awsconfig.WithRegion(region),
		awsconfig.WithCredentialsProvider(
			credentials.NewStaticCredentialsProvider(cfg.AccessKeyID, cfg.SecretAccessKey, ""),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("image store: load aws config: %w", err)
	}

	client := s3.NewFromConfig(awsCfg, func(o *s3.Options) {
		if cfg.Endpoint != "" {
			o.BaseEndpoint = &cfg.Endpoint
		}
		if cfg.ForcePathStyle {
			o.UsePathStyle = true
		}
		o.APIOptions = append(o.APIOptions, v4.SwapComputePayloadSHA256ForUnsignedPayloadMiddleware)
		o.RequestChecksumCalculation = aws.RequestChecksumCalculationWhenRequired
	})

	prefix := strings.Trim(strings.TrimSpace(cfg.Prefix), "/")
	return &s3ImageStore{client: client, bucket: cfg.Bucket, prefix: prefix}, nil
}

// S3ImageStoreConfig configures the S3 image store (mirrors config.ImageStudioS3Config).
type S3ImageStoreConfig struct {
	Endpoint        string
	Region          string
	Bucket          string
	AccessKeyID     string
	SecretAccessKey string
	Prefix          string
	ForcePathStyle  bool
}

// objectKey applies the optional prefix to a storage key. The returned key (the
// caller-visible storage key) does NOT include the prefix, so keys stay stable
// even if the prefix changes; only the S3 object name is prefixed.
func (s *s3ImageStore) objectKey(key string) string {
	if s.prefix == "" {
		return key
	}
	return path.Join(s.prefix, key)
}

func (s *s3ImageStore) Put(ctx context.Context, userID, genID int64, idx int, contentType string, data []byte) (string, error) {
	key := imageStudioOutputKey(userID, genID, idx, contentType)
	return s.putObject(ctx, key, contentType, data)
}

func (s *s3ImageStore) PutInput(ctx context.Context, userID, genID int64, idx int, contentType string, data []byte) (string, error) {
	key := imageStudioInputKey(userID, genID, idx, contentType)
	return s.putObject(ctx, key, contentType, data)
}

func (s *s3ImageStore) putObject(ctx context.Context, key, contentType string, data []byte) (string, error) {
	ct := strings.TrimSpace(contentType)
	if ct == "" {
		// Infer from the key's extension so Open returns a consistent type.
		ct = contentTypeForExt(strings.TrimPrefix(path.Ext(key), "."))
	}
	objectKey := s.objectKey(key)
	if _, err := s.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:      &s.bucket,
		Key:         &objectKey,
		Body:        bytes.NewReader(data),
		ContentType: &ct,
	}); err != nil {
		return "", fmt.Errorf("image store: s3 put %q: %w", key, err)
	}
	return key, nil
}

func (s *s3ImageStore) Open(ctx context.Context, key string) (io.ReadCloser, string, error) {
	objectKey := s.objectKey(key)
	out, err := s.client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: &s.bucket,
		Key:    &objectKey,
	})
	if err != nil {
		return nil, "", fmt.Errorf("image store: s3 open %q: %w", key, err)
	}
	// Prefer the extension-derived type (stable, 1-to-1 with the key) over the
	// stored header, matching localImageStore.Open's behavior.
	ct := contentTypeForExt(strings.TrimPrefix(path.Ext(key), "."))
	if ct == "" && out.ContentType != nil {
		ct = *out.ContentType
	}
	return out.Body, ct, nil
}

func (s *s3ImageStore) Delete(ctx context.Context, key string) error {
	objectKey := s.objectKey(key)
	if _, err := s.client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: &s.bucket,
		Key:    &objectKey,
	}); err != nil {
		// Idempotent: a missing object is not an error (mirrors localImageStore).
		if isS3NotFound(err) {
			return nil
		}
		return fmt.Errorf("image store: s3 delete %q: %w", key, err)
	}
	return nil
}

// isS3NotFound reports whether err is an S3 "no such key" / 404 response.
func isS3NotFound(err error) bool {
	var apiErr smithy.APIError
	if errors.As(err, &apiErr) {
		switch apiErr.ErrorCode() {
		case "NoSuchKey", "NotFound":
			return true
		}
	}
	var respErr interface{ HTTPStatusCode() int }
	if errors.As(err, &respErr) && respErr.HTTPStatusCode() == http.StatusNotFound {
		return true
	}
	return false
}

var _ ImageStore = (*s3ImageStore)(nil)
