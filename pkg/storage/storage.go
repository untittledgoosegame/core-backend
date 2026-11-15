package storage

import (
	"bytes"
	"context"
	"fmt"
	"strings"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

// Config targets any S3-compatible store: Cloudflare R2 in production, MinIO locally.
type Config struct {
	Endpoint  string `mapstructure:"endpoint"`
	Region    string `mapstructure:"region"`
	Bucket    string `mapstructure:"bucket"`
	AccessKey string `mapstructure:"access_key"`
	SecretKey string `mapstructure:"secret_key"`
	UseSSL    bool   `mapstructure:"use_ssl"`
	PublicURL string `mapstructure:"public_url"`
}

// Client uploads through the S3 API endpoint; files are served from PublicURL, a separate domain.
type Client struct {
	mc        *minio.Client
	bucket    string
	publicURL string
}

// New returns nil when storage is not configured, so callers can report uploads as unavailable.
func New(ctx context.Context, cfg Config) (*Client, error) {
	if cfg.Endpoint == "" {
		return nil, nil
	}
	if cfg.Bucket == "" || cfg.PublicURL == "" {
		return nil, fmt.Errorf("storage bucket and public_url are required when an endpoint is set")
	}
	endpoint := strings.TrimPrefix(strings.TrimPrefix(cfg.Endpoint, "https://"), "http://")
	mc, err := minio.New(endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(cfg.AccessKey, cfg.SecretKey, ""),
		Secure: cfg.UseSSL,
		Region: cfg.Region,
	})
	if err != nil {
		return nil, fmt.Errorf("storage client: %w", err)
	}
	if ok, err := mc.BucketExists(ctx, cfg.Bucket); err != nil || !ok {
		return nil, fmt.Errorf("storage bucket %q not reachable (exists=%v): %v", cfg.Bucket, ok, err)
	}
	return &Client{mc: mc, bucket: cfg.Bucket, publicURL: strings.TrimRight(cfg.PublicURL, "/")}, nil
}

func (c *Client) URL(key string) string { return c.publicURL + "/" + key }

// Put stores an immutable public object; keys must be unique per version since CDNs cache them forever.
func (c *Client) Put(ctx context.Context, key, contentType string, data []byte) (string, error) {
	_, err := c.mc.PutObject(ctx, c.bucket, key, bytes.NewReader(data), int64(len(data)), minio.PutObjectOptions{
		ContentType:  contentType,
		CacheControl: "public, max-age=31536000, immutable",
	})
	if err != nil {
		return "", fmt.Errorf("storage put: %w", err)
	}
	return c.URL(key), nil
}

func (c *Client) Delete(ctx context.Context, key string) error {
	return c.mc.RemoveObject(ctx, c.bucket, key, minio.RemoveObjectOptions{})
}
