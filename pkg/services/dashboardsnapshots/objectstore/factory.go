package objectstore

import "fmt"

// NewObjectStorage creates an ObjectStorage implementation based on the provider config
func NewObjectStorage(cfg Config) (ObjectStorage, error) {
	switch cfg.Provider {
	case "tencent":
		return NewCOSStorage(cfg)
	default:
		return nil, fmt.Errorf("unsupported object storage provider: %s", cfg.Provider)
	}
}
