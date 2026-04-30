package storage

import (
	"context"
	"fmt"
	"time"

	"github.com/CodeEnthusiast09/proctura-backend/internal/config"
	miniogo "github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

type minioProvider struct {
	client    *miniogo.Client
	bucket    string
	publicURL string
}

// NewMinIOProvider returns nil (and no error) when MinIO is not configured,
// so callers can treat a nil *minioProvider as "MinIO disabled".
func NewMinIOProvider(cfg config.MinIOConfig) (*minioProvider, error) {
	if cfg.Endpoint == "" || cfg.AccessKey == "" || cfg.SecretKey == "" {
		return nil, nil
	}
	client, err := miniogo.New(cfg.Endpoint, &miniogo.Options{
		Creds:  credentials.NewStaticV4(cfg.AccessKey, cfg.SecretKey, ""),
		Secure: cfg.UseSSL,
	})
	if err != nil {
		return nil, fmt.Errorf("minio init: %w", err)
	}
	return &minioProvider{
		client:    client,
		bucket:    cfg.Bucket,
		publicURL: cfg.PublicURL,
	}, nil
}

func (p *minioProvider) token(submissionID string) (*UploadToken, error) {
	objectKey := fmt.Sprintf("recordings/%s.webm", submissionID)

	presigned, err := p.client.PresignedPutObject(
		context.Background(),
		p.bucket,
		objectKey,
		15*time.Minute,
	)
	if err != nil {
		return nil, fmt.Errorf("minio presign: %w", err)
	}

	publicURL := fmt.Sprintf("%s/%s/%s", p.publicURL, p.bucket, objectKey)

	return &UploadToken{
		Provider: "minio",
		MinIO: &MinIOToken{
			UploadURL: presigned.String(),
			PublicURL: publicURL,
		},
	}, nil
}
