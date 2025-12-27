package objectstore

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"

	"github.com/tencentyun/cos-go-sdk-v5"
)

// COSStorage implements ObjectStorage using Tencent Cloud COS
type COSStorage struct {
	client *cos.Client
	bucket string
}

// NewCOSStorage creates a new COS storage backend
func NewCOSStorage(cfg Config) (*COSStorage, error) {
	if cfg.Bucket == "" {
		return nil, fmt.Errorf("bucket is required for COS storage")
	}
	if cfg.SecretID == "" || cfg.SecretKey == "" {
		return nil, fmt.Errorf("secret_id and secret_key are required for COS storage")
	}

	// Build bucket URL: https://<bucket>-<appid>.cos.<region>.myqcloud.com
	bucketURL, err := url.Parse(cfg.Endpoint)
	if err != nil {
		return nil, fmt.Errorf("invalid endpoint: %w", err)
	}

	client := cos.NewClient(&cos.BaseURL{BucketURL: bucketURL}, &http.Client{
		Transport: &cos.AuthorizationTransport{
			SecretID:  cfg.SecretID,
			SecretKey: cfg.SecretKey,
		},
	})

	return &COSStorage{
		client: client,
		bucket: cfg.Bucket,
	}, nil
}

// Put uploads data to COS
func (c *COSStorage) Put(ctx context.Context, key string, data io.Reader, size int64) error {
	opt := &cos.ObjectPutOptions{
		ObjectPutHeaderOptions: &cos.ObjectPutHeaderOptions{
			ContentLength: size,
		},
	}
	_, err := c.client.Object.Put(ctx, key, data, opt)
	return err
}

// Get retrieves data from COS
func (c *COSStorage) Get(ctx context.Context, key string) (io.ReadCloser, error) {
	resp, err := c.client.Object.Get(ctx, key, nil)
	if err != nil {
		return nil, err
	}
	return resp.Body, nil
}

// Delete removes an object from COS
func (c *COSStorage) Delete(ctx context.Context, key string) error {
	_, err := c.client.Object.Delete(ctx, key)
	return err
}

// Exists checks if an object exists in COS
func (c *COSStorage) Exists(ctx context.Context, key string) (bool, error) {
	return c.client.Object.IsExist(ctx, key)
}

// List returns all keys with the specified prefix
func (c *COSStorage) List(ctx context.Context, prefix string) ([]string, error) {
	var keys []string
	var marker string

	for {
		opt := &cos.BucketGetOptions{
			Prefix:  prefix,
			Marker:  marker,
			MaxKeys: 1000,
		}

		result, _, err := c.client.Bucket.Get(ctx, opt)
		if err != nil {
			return nil, err
		}

		for _, obj := range result.Contents {
			keys = append(keys, obj.Key)
		}

		if !result.IsTruncated {
			break
		}
		marker = result.NextMarker
	}

	return keys, nil
}

// Ensure COSStorage implements ObjectStorage
var _ ObjectStorage = (*COSStorage)(nil)
