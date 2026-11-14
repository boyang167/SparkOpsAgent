package sftpgateway

import (
	"os"
	"strings"

	"github.com/google/uuid"

	"github.com/sparkops/sparkops-agent/backend/internal/platform/config"
)

func authFromConfig(cfg *config.Config) TokenValidator {
	token := strings.TrimSpace(os.Getenv("SPARKOPS_SFTP_DEV_TOKEN"))
	if token == "" {
		return UnconfiguredTokenValidator{}
	}
	org := envUUID("SPARKOPS_SFTP_ORG_ID")
	ws := envUUID("SPARKOPS_SFTP_WORKSPACE_ID")
	actor := envUUID("SPARKOPS_SFTP_ACTOR_ID")
	if org == uuid.Nil || ws == uuid.Nil || actor == uuid.Nil {
		return UnconfiguredTokenValidator{}
	}
	return StaticTokenValidator{
		Token: token,
		Principal: Principal{
			OrganizationID: org, WorkspaceID: ws, ActorID: actor,
		},
	}
}

func controlFromConfig(cfg *config.Config) (TransferControlPort, error) {
	sftp := cfg.Filesystem.SFTP
	base := strings.TrimSpace(sftp.ControlBaseURL)
	if base == "" {
		base = strings.TrimSpace(cfg.Filesystem.TransferGateway.ControlBaseURL)
	}
	if base == "" {
		return UnconfiguredTransferControl{}, nil
	}
	token := strings.TrimSpace(sftp.InternalToken)
	if token == "" {
		token = strings.TrimSpace(cfg.Filesystem.TransferGateway.InternalToken)
	}
	if token == "" {
		token = cfg.JWT.Secret
	}
	storageRoot := strings.TrimSpace(cfg.Filesystem.TransferGateway.StorageRoot)
	return NewHTTPTransferControl(base, token, storageRoot)
}

func envUUID(key string) uuid.UUID {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return uuid.Nil
	}
	id, err := uuid.Parse(raw)
	if err != nil {
		return uuid.Nil
	}
	return id
}
