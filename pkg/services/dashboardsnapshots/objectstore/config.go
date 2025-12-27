package objectstore

// Config holds the configuration for object storage
type Config struct {
	// Provider specifies the object storage provider (tencent, s3, oss, minio)
	Provider string `json:"provider"`

	// Bucket is the bucket/container name
	Bucket string `json:"bucket"`

	// Region is the region of the bucket
	Region string `json:"region"`

	// Endpoint is the custom endpoint URL (required for COS, MinIO)
	Endpoint string `json:"endpoint"`

	// SecretID is the access key ID
	SecretID string `json:"secret_id"`

	// SecretKey is the secret access key
	SecretKey string `json:"secret_key"`

	// PathPrefix is the prefix for all snapshot objects
	PathPrefix string `json:"path_prefix"`
}

// DefaultConfig returns a default configuration
func DefaultConfig() Config {
	return Config{
		Provider:   "tencent",
		PathPrefix: "snapshots/",
	}
}
