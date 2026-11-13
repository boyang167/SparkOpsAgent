package mcp

import (
	"fmt"

	fscommand "github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/application/command"
	fsquery "github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/application/query"
	domainservice "github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/domain/service"
	agentsign "github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/infrastructure/capability"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/infrastructure/persistence"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/mcp/application"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/mcp/domain"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/appctx"
)

func registerFilesystemTools(registry *domain.Registry, c *appctx.Container) error {
	if registry == nil || c == nil || c.DB == nil {
		return nil
	}
	secret, audience := agentCapabilitySigningMaterial(c)
	signer, err := agentsign.NewSigner(secret, audience)
	if err != nil {
		return fmt.Errorf("mcp filesystem capability signer: %w", err)
	}
	repo := persistence.NewRepository(c.DB)
	capStore := persistence.NewCapabilityRepository(c.DB)
	createTransfer := fscommand.NewCreateTransferHandler(repo, c.Transactor, c.EventBus, c.AuditLogger, nil)
	application.RegisterFilesystemTools(registry, application.FilesystemToolDeps{
		CapabilityStore: capStore,
		Verifier:        signer,
		GetNamespace:    repo.GetNamespaceByWorkspace,
		ListChildren:    fsquery.NewListChildrenHandler(repo),
		GetNode:         repo.GetNodeByID,
		GetRevision:     repo.GetRevision,
		CreateTransfer:  createTransfer.Handle,
		CreateArtifact:  fscommand.NewCreateArtifactHandler(repo, c.Transactor, c.EventBus).Handle,
	})
	return nil
}

func agentCapabilitySigningMaterial(c *appctx.Container) (secret, audience string) {
	audience = domainservice.AgentCapabilityAudience
	if c.Config == nil {
		return "filesystem-transfer-dev", audience
	}
	tg := c.Config.Filesystem.TransferGateway
	secret = tg.CapabilitySecret
	if secret == "" {
		secret = c.Config.JWT.Secret
	}
	if secret == "" {
		secret = "filesystem-transfer-dev"
	}
	return secret, audience
}
