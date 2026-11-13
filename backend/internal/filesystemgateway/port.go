package filesystemgateway

import (
	"context"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/storage"
)

// ControlPort resolves upload/download grants from the control plane.
// The gateway must not infer object keys or metadata on its own.
type ControlPort interface {
	ResolveUpload(ctx context.Context, sessionID string) (UploadGrant, error)
	ResolveDownload(ctx context.Context, fileID string) (DownloadGrant, error)
}

type UploadGrant struct {
	TemporaryObjectKey string
	ExpectedSize       int64
	Provider           storage.Provider
	Classification     model.TransferClassification
}

type DownloadGrant struct {
	ObjectKey      string
	Provider       storage.Provider
	Classification model.TransferClassification
	AuditOK        bool
}
