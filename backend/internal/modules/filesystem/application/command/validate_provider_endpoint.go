package command

import (
	"os"
	"strings"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/domain"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/domain/model"
)

func validateProviderEndpoint(providerType model.ProviderType, endpoint string) error {
	endpoint = strings.TrimSpace(endpoint)
	switch providerType {
	case model.ProviderTypeLocal, model.ProviderTypeNFS:
		if endpoint == "" {
			return model.ErrInvalidStorageProvider
		}
		info, err := os.Stat(endpoint)
		if err != nil {
			if os.IsNotExist(err) {
				return domain.ErrStorageProviderEndpointNotFound
			}
			return err
		}
		if !info.IsDir() {
			return domain.ErrStorageProviderEndpointNotADirectory
		}
	case model.ProviderTypeS3:
		if endpoint == "" {
			return model.ErrInvalidStorageProvider
		}
	}
	return nil
}