package server

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

// MinIOConfig holds the connection parameters for a MinIO instance.
// Fields map to the "minio" section of ~/.soul/config.json.
type MinIOConfig struct {
	Endpoint  string `json:"endpoint"`
	AccessKey string `json:"access_key"`
	SecretKey string `json:"secret_key"`
	Bucket    string `json:"bucket"`
	UseSSL    bool   `json:"use_ssl"`
}

// soulConfig is the top-level structure of ~/.soul/config.json.
type soulConfig struct {
	MinIO MinIOConfig `json:"minio"`
}

// LoadMinIOConfig reads ~/.soul/config.json and returns the MinIO configuration.
func LoadMinIOConfig() (*MinIOConfig, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("minio: resolve home dir: %w", err)
	}
	path := filepath.Join(home, ".soul", "config.json")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("minio: read config %s: %w", path, err)
	}
	var sc soulConfig
	if err := json.Unmarshal(data, &sc); err != nil {
		return nil, fmt.Errorf("minio: parse config: %w", err)
	}
	return &sc.MinIO, nil
}

// MinIOClient wraps a minio.Client with a default bucket name.
type MinIOClient struct {
	client *minio.Client
	bucket string
}

// NewMinIOClient creates a MinIO client from the given configuration.
func NewMinIOClient(cfg *MinIOConfig) (*MinIOClient, error) {
	client, err := minio.New(cfg.Endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(cfg.AccessKey, cfg.SecretKey, ""),
		Secure: cfg.UseSSL,
	})
	if err != nil {
		return nil, fmt.Errorf("minio: create client: %w", err)
	}
	return &MinIOClient{
		client: client,
		bucket: cfg.Bucket,
	}, nil
}

// Upload stores an object in the configured bucket under the given key.
func (m *MinIOClient) Upload(ctx context.Context, key, contentType string, reader io.Reader, size int64) error {
	_, err := m.client.PutObject(ctx, m.bucket, key, reader, size, minio.PutObjectOptions{
		ContentType: contentType,
	})
	if err != nil {
		return fmt.Errorf("minio: upload %s: %w", key, err)
	}
	return nil
}

// PresignedURL returns a pre-signed GET URL for the object, valid for 15 minutes.
func (m *MinIOClient) PresignedURL(ctx context.Context, key string) (string, error) {
	u, err := m.client.PresignedGetObject(ctx, m.bucket, key, 15*time.Minute, nil)
	if err != nil {
		return "", fmt.Errorf("minio: presign %s: %w", key, err)
	}
	return u.String(), nil
}

// Download retrieves an object from the configured bucket.
// The caller is responsible for closing the returned ReadCloser.
func (m *MinIOClient) Download(ctx context.Context, key string) (io.ReadCloser, error) {
	obj, err := m.client.GetObject(ctx, m.bucket, key, minio.GetObjectOptions{})
	if err != nil {
		return nil, fmt.Errorf("minio: download %s: %w", key, err)
	}
	return obj, nil
}
