package storage

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"
)

// Uploader stores encrypted snapshot blobs in object storage.
type Uploader interface {
	Upload(ctx context.Context, key string, data []byte) (uri string, err error)
	PresignGet(ctx context.Context, key string, expiry time.Duration) (string, error)
}

// Config from environment.
type Config struct {
	S3Bucket   string
	S3Prefix   string
	S3Region   string
	S3Endpoint string
	GCSBucket  string
	GCSPrefix  string
}

func ConfigFromEnv() Config {
	return Config{
		S3Bucket:   os.Getenv("DRE_S3_BUCKET"),
		S3Prefix:   strings.Trim(os.Getenv("DRE_S3_PREFIX"), "/"),
		S3Region:   envOr("DRE_S3_REGION", "us-east-1"),
		S3Endpoint: os.Getenv("DRE_S3_ENDPOINT"),
		GCSBucket:  os.Getenv("DRE_GCS_BUCKET"),
		GCSPrefix:  strings.Trim(os.Getenv("DRE_GCS_PREFIX"), "/"),
	}
}

func NewFromConfig(cfg Config) (Uploader, error) {
	if cfg.GCSBucket != "" {
		return newGCS(cfg)
	}
	if cfg.S3Bucket != "" {
		return newS3(cfg)
	}
	return nil, nil
}

func ObjectKey(prefix, snapshotID string) string {
	name := fmt.Sprintf("incident-%s.dre", snapshotID)
	if prefix == "" {
		return name
	}
	return prefix + "/" + name
}

func envOr(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}
