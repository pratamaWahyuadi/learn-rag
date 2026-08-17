// Package r2 implements ports.Storage against Cloudflare R2 (S3-compatible)
// using the AWS SDK for Go v2. Object keys and presigned URLs are never logged.
package r2

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/feature/s3/manager"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"

	"github.com/pratamaWahyuadi/learn-rag/internal/core/ports"
)

// Config holds the settings required to talk to a Cloudflare R2 bucket.
type Config struct {
	AccountID       string
	AccessKeyID     string
	SecretAccessKey string
	Bucket          string
	// PublicEndpoint is the optional public R2 endpoint for browser uploads.
	PublicEndpoint string
}

// Storage implements ports.Storage backed by Cloudflare R2.
type Storage struct {
	client *s3.Client
	bucket string
}

// Compile-time assertion that Storage satisfies ports.Storage.
var _ ports.Storage = (*Storage)(nil)

// New builds a Storage client using the provided credentials and endpoint.
func New(cfg Config) (*Storage, error) {
	awsCfg, err := awsconfig.LoadDefaultConfig(
		context.Background(),
		awsconfig.WithRegion("auto"),
		awsconfig.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(
			cfg.AccessKeyID,
			cfg.SecretAccessKey,
			"",
		)),
		awsconfig.WithBaseEndpoint(fmt.Sprintf("https://%s.r2.cloudflarestorage.com", cfg.AccountID)),
	)
	if err != nil {
		return nil, fmt.Errorf("r2: configure aws sdk: %w", err)
	}
	client := s3.NewFromConfig(awsCfg)
	return &Storage{client: client, bucket: cfg.Bucket}, nil
}

// GenerateUploadURL returns a presigned PUT URL for fileKey that expires at
// expiresAt (expected 5–10 minutes).
func (s *Storage) GenerateUploadURL(ctx context.Context, fileKey, contentType string, expiresAt time.Time) (string, error) {
	presigner := s3.NewPresignClient(s.client)
	req, err := presigner.PresignPutObject(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(s.bucket),
		Key:         aws.String(fileKey),
		ContentType: aws.String(contentType),
	}, func(o *s3.PresignOptions) {
		o.Expires = time.Until(expiresAt)
	})
	if err != nil {
		return "", fmt.Errorf("r2: presign upload: %w", err)
	}
	return req.URL, nil
}

// Download writes the object identified by fileKey to destPath on disk.
func (s *Storage) Download(ctx context.Context, fileKey, destPath string) error {
	file, err := os.Create(destPath)
	if err != nil {
		return fmt.Errorf("r2: create download file: %w", err)
	}
	defer file.Close()

	downloader := manager.NewDownloader(s.client)
	if _, err := downloader.Download(ctx, file, &s3.GetObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(fileKey),
	}); err != nil {
		return fmt.Errorf("r2: download: %w", err)
	}
	return nil
}

// Delete removes the object identified by fileKey. A missing object
// (NoSuchKey) is treated as success so cleanup stays idempotent.
func (s *Storage) Delete(ctx context.Context, fileKey string) error {
	_, err := s.client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(fileKey),
	})
	if err != nil {
		var nsk *types.NoSuchKey
		if errors.As(err, &nsk) {
			return nil
		}
		return fmt.Errorf("r2: delete: %w", err)
	}
	return nil
}
