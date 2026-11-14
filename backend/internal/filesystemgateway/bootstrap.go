package filesystemgateway

import (
	"context"
	"fmt"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/sparkops/sparkops-agent/backend/internal/platform/config"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/storage/adapters/local"
)

// UnconfiguredControlPort fails grant resolution until the control plane adapter is wired.
type UnconfiguredControlPort struct{}

func (UnconfiguredControlPort) ResolveUpload(_ context.Context, _ string) (UploadGrant, error) {
	return UploadGrant{}, fmt.Errorf("%w", ErrControlUnavailable)
}

func (UnconfiguredControlPort) ResolveDownload(_ context.Context, _ string) (DownloadGrant, error) {
	return DownloadGrant{}, fmt.Errorf("%w", ErrControlUnavailable)
}

func (UnconfiguredControlPort) RecordUploaded(context.Context, string, int64, string) error {
	return fmt.Errorf("%w", ErrControlUnavailable)
}

// NewEngineFromConfig builds a gin engine for the transfer gateway binary.
func NewEngineFromConfig(cfg *config.Config) (*gin.Engine, error) {
	if cfg == nil {
		return nil, fmt.Errorf("%w: nil config", ErrInvalidGatewayConfig)
	}
	tg := cfg.Filesystem.TransferGateway
	provider, err := local.New(tg.StorageRoot)
	if err != nil {
		return nil, fmt.Errorf("storage root: %w", err)
	}
	control := ControlPort(UnconfiguredControlPort{})
	if baseURL := strings.TrimSpace(tg.ControlBaseURL); baseURL != "" {
		token := strings.TrimSpace(tg.InternalToken)
		if token == "" {
			token = cfg.JWT.Secret
		}
		httpControl, err := NewHTTPControlPort(baseURL, token, provider)
		if err != nil {
			return nil, err
		}
		control = httpControl
	}
	_ = provider.Close()

	server, err := New(Options{
		CapabilitySecret:           tg.CapabilitySecret,
		CapabilityAudience:         tg.CapabilityAudience,
		CapabilitySecretFallback:   cfg.JWT.Secret,
		MaxFileBytes:               tg.MaxFileBytes,
		Control:                    control,
	})
	if err != nil {
		return nil, err
	}
	engine := gin.New()
	engine.Use(gin.Recovery())
	server.RegisterRoutes(engine)
	return engine, nil
}
