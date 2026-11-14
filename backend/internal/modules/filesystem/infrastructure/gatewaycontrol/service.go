package gatewaycontrol

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"github.com/sparkops/sparkops-agent/backend/internal/filesystemgateway"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/application/command"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/domain"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/domain/repository"
)

type Service struct {
	repo     repository.FilesystemRepository
	catalog  command.StorageProviderCatalog
	recorded func(context.Context, command.RecordTransferUploaded) error
}

func NewService(
	repo repository.FilesystemRepository,
	catalog command.StorageProviderCatalog,
	recorded func(context.Context, command.RecordTransferUploaded) error,
) *Service {
	return &Service{repo: repo, catalog: catalog, recorded: recorded}
}

func (s *Service) ResolveUpload(ctx context.Context, sessionID string) (filesystemgateway.UploadGrant, error) {
	if s == nil || s.repo == nil || s.catalog == nil {
		return filesystemgateway.UploadGrant{}, fmt.Errorf("%w", filesystemgateway.ErrControlUnavailable)
	}
	id, err := uuid.Parse(sessionID)
	if err != nil {
		return filesystemgateway.UploadGrant{}, domain.ErrTransferNotFound
	}
	session, err := s.repo.GetTransferByID(ctx, id)
	if err != nil {
		return filesystemgateway.UploadGrant{}, err
	}
	providerID := session.ProviderID()
	if providerID == nil {
		return filesystemgateway.UploadGrant{}, domain.ErrTransferNotFound
	}
	provider, err := s.catalog.Provider(ctx, *providerID)
	if err != nil {
		return filesystemgateway.UploadGrant{}, err
	}
	return filesystemgateway.UploadGrant{
		TemporaryObjectKey: session.TemporaryObjectKey(),
		ExpectedSize:       session.ExpectedSize(),
		Provider:           provider,
		Classification:     session.Classification(),
	}, nil
}

func (s *Service) ResolveDownload(ctx context.Context, fileID string) (filesystemgateway.DownloadGrant, error) {
	return filesystemgateway.DownloadGrant{}, fmt.Errorf("%w", filesystemgateway.ErrControlUnavailable)
}

func (s *Service) RecordUploaded(ctx context.Context, sessionID string, size int64, sha256 string) error {
	if s.recorded == nil {
		return fmt.Errorf("%w", filesystemgateway.ErrControlUnavailable)
	}
	id, err := uuid.Parse(sessionID)
	if err != nil {
		return domain.ErrTransferNotFound
	}
	session, err := s.repo.GetTransferByID(ctx, id)
	if err != nil {
		return err
	}
	return s.recorded(ctx, command.RecordTransferUploaded{
		SessionID: id, WorkspaceID: session.WorkspaceID(), Size: size, SHA256: sha256,
	})
}

var _ filesystemgateway.ControlPort = (*Service)(nil)
