package storage

import (
	"bytes"
	"context"
	"fmt"
	"time"

	"cloud.google.com/go/storage"
)

type gcsUploader struct {
	client *storage.Client
	bucket string
	prefix string
}

func newGCS(cfg Config) (Uploader, error) {
	ctx := context.Background()
	client, err := storage.NewClient(ctx)
	if err != nil {
		return nil, err
	}
	return &gcsUploader{client: client, bucket: cfg.GCSBucket, prefix: cfg.GCSPrefix}, nil
}

func (u *gcsUploader) Upload(ctx context.Context, key string, data []byte) (string, error) {
	fullKey := ObjectKey(u.prefix, trimIncidentKey(key))
	w := u.client.Bucket(u.bucket).Object(fullKey).NewWriter(ctx)
	w.ContentType = "application/octet-stream"
	if _, err := w.Write(data); err != nil {
		_ = w.Close()
		return "", err
	}
	if err := w.Close(); err != nil {
		return "", err
	}
	return fmt.Sprintf("gs://%s/%s", u.bucket, fullKey), nil
}

func (u *gcsUploader) PresignGet(ctx context.Context, key string, expiry time.Duration) (string, error) {
	fullKey := ObjectKey(u.prefix, trimIncidentKey(key))
	return u.client.Bucket(u.bucket).SignedURL(fullKey, &storage.SignedURLOptions{
		Method:  "GET",
		Expires: time.Now().Add(expiry),
	})
}
