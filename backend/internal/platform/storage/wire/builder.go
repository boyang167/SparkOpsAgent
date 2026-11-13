package wire

import (
	"encoding/json"
	"fmt"

	"github.com/sparkops/sparkops-agent/backend/internal/platform/storage"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/storage/adapters/local"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/storage/adapters/nfs"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/storage/adapters/s3"
)

type s3Secret struct {
	Bucket    string `json:"bucket"`
	AccessKey string `json:"access_key"`
	SecretKey string `json:"secret_key"`
	Region    string `json:"region,omitempty"`
	PathStyle bool   `json:"path_style,omitempty"`
}

func DefaultProviderBuilder(record storage.ProviderRecord, secret json.RawMessage) (storage.Provider, error) {
	switch record.ProviderType {
	case storage.ProviderTypeLocal:
		if record.Endpoint == "" {
			return nil, fmt.Errorf("%w: empty local root", storage.ErrInvalidRoot)
		}
		return local.New(record.Endpoint)
	case storage.ProviderTypeNFS:
		if record.Endpoint == "" {
			return nil, fmt.Errorf("%w: empty nfs root", storage.ErrInvalidRoot)
		}
		return nfs.New(record.Endpoint)
	case storage.ProviderTypeS3:
		var creds s3Secret
		if len(secret) > 0 {
			if err := json.Unmarshal(secret, &creds); err != nil {
				return nil, err
			}
		}
		bucket := creds.Bucket
		if bucket == "" {
			bucket = record.Endpoint
		}
		return s3.New(s3.Config{
			Endpoint:  record.Endpoint,
			Region:    creds.Region,
			Bucket:    bucket,
			PathStyle: creds.PathStyle,
			AccessKey: creds.AccessKey,
			SecretKey: creds.SecretKey,
		})
	default:
		return nil, storage.ErrUnsupportedProviderType
	}
}
