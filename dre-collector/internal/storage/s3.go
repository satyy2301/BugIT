package storage

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

type s3Uploader struct {
	client *s3.Client
	bucket string
	prefix string
}

func newS3(cfg Config) (Uploader, error) {
	loadOpts := []func(*config.LoadOptions) error{
		config.WithRegion(cfg.S3Region),
	}
	if cfg.S3Endpoint != "" {
		custom := aws.EndpointResolverWithOptionsFunc(func(service, region string, _ ...interface{}) (aws.Endpoint, error) {
			return aws.Endpoint{URL: cfg.S3Endpoint, HostnameImmutable: true}, nil
		})
		loadOpts = append(loadOpts, config.WithEndpointResolverWithOptions(custom))
	}
	if osAccessKey := os.Getenv("AWS_ACCESS_KEY_ID"); osAccessKey != "" {
		loadOpts = append(loadOpts, config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(
			osAccessKey, os.Getenv("AWS_SECRET_ACCESS_KEY"), os.Getenv("AWS_SESSION_TOKEN"),
		)))
	}
	awsCfg, err := config.LoadDefaultConfig(context.Background(), loadOpts...)
	if err != nil {
		return nil, err
	}
	client := s3.NewFromConfig(awsCfg, func(o *s3.Options) {
		if cfg.S3Endpoint != "" {
			o.UsePathStyle = true
		}
	})
	return &s3Uploader{client: client, bucket: cfg.S3Bucket, prefix: cfg.S3Prefix}, nil
}

func (u *s3Uploader) Upload(ctx context.Context, key string, data []byte) (string, error) {
	fullKey := ObjectKey(u.prefix, trimIncidentKey(key))
	_, err := u.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(u.bucket),
		Key:         aws.String(fullKey),
		Body:        bytes.NewReader(data),
		ContentType: aws.String("application/octet-stream"),
	})
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("s3://%s/%s", u.bucket, fullKey), nil
}

func (u *s3Uploader) PresignGet(ctx context.Context, key string, expiry time.Duration) (string, error) {
	fullKey := ObjectKey(u.prefix, trimIncidentKey(key))
	presigner := s3.NewPresignClient(u.client)
	out, err := presigner.PresignGetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(u.bucket),
		Key:    aws.String(fullKey),
	}, s3.WithPresignExpires(expiry))
	if err != nil {
		return "", err
	}
	return out.URL, nil
}

func trimIncidentKey(key string) string {
	key = strings.TrimPrefix(key, "incident-")
	key = strings.TrimSuffix(key, ".dre")
	return key
}
