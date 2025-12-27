package objectstore

import (
	"context"
	"io"
)

// ObjectStorage defines the interface for object storage backends.
// Implementations can be COS, S3, OSS, MinIO, etc.
type ObjectStorage interface {
	// Put uploads data to the specified key
	Put(ctx context.Context, key string, data io.Reader, size int64) error

	// Get retrieves data from the specified key
	Get(ctx context.Context, key string) (io.ReadCloser, error)

	// Delete removes the object at the specified key
	Delete(ctx context.Context, key string) error

	// Exists checks if an object exists at the specified key
	Exists(ctx context.Context, key string) (bool, error)

	// List returns all keys with the specified prefix
	List(ctx context.Context, prefix string) ([]string, error)
}
