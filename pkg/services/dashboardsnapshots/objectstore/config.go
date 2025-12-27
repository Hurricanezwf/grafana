package objectstore

// Config holds the configuration for object storage
type Config struct {
	// Provider specifies the object storage provider (tencent, s3, oss, minio)
	Provider string `json:"provider"`

	// Endpoint is the bucket endpoint URL
	// For Tencent COS: https://<bucket>.cos.<region>.myqcloud.com
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
		PathPrefix: "never-delete-me/",
	}
}
