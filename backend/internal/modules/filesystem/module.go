package filesystem

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	resourceauth "github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/application/authorization"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/application/capability"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/application/command"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/application/operation"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/application/query"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/application/subscriber"
	filesystemevent "github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/domain/event"
	domainservice "github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/domain/service"
	agentsign "github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/infrastructure/capability"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/infrastructure/gatewaycontrol"
	fsstorage "github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/infrastructure/storage"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/infrastructure/persistence"
	filesystemhttp "github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/interface/http"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/appctx"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/audit"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/authn"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/authz"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/entitlements"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/integration"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

type Module struct {
	tokens *authn.TokenProvider
}

func New(tokens *authn.TokenProvider) *Module {
	return &Module{tokens: tokens}
}

func (m *Module) Name() string { return "filesystem" }

func (m *Module) Register(c *appctx.Container) error {
	if c == nil {
		return fmt.Errorf("filesystem module requires container")
	}
	if c.DB == nil {
		return fmt.Errorf("filesystem module requires database")
	}
	if c.Transactor == nil {
		return fmt.Errorf("filesystem module requires transactor")
	}
	if c.EventBus == nil {
		return fmt.Errorf("filesystem module requires event bus")
	}
	if c.PermissionChecker == nil {
		return fmt.Errorf("filesystem module requires permission checker")
	}
	if c.WorkspaceMemberChecker == nil {
		return fmt.Errorf("filesystem module requires workspace member checker")
	}
	if c.AuditLogger == nil {
		auditStore := audit.NewGormStore(c.DB)
		if err := auditStore.AutoMigrate(); err != nil {
			return fmt.Errorf("filesystem audit migrate: %w", err)
		}
		c.AuditLogger = audit.NewService(auditStore)
	}
	if c.EventRegistry == nil {
		return fmt.Errorf("filesystem module requires event registry")
	}
	if c.Engine == nil {
		return fmt.Errorf("filesystem module requires HTTP engine")
	}
	if err := filesystemevent.Register(c.EventRegistry); err != nil {
		return fmt.Errorf("filesystem event codecs: %w", err)
	}
	if err := persistence.AutoMigrate(c.DB); err != nil {
		return fmt.Errorf("filesystem migrate: %w", err)
	}

	repo := persistence.NewRepository(c.DB)
	resourceAuthorizer := resourceauth.NewService(repo, func(ctx context.Context, workspaceID, actorID uuid.UUID) (bool, error) {
		return c.PermissionChecker.HasPermission(ctx, kernel.WorkspaceID{UUID: workspaceID}, kernel.MemberID{UUID: actorID}, authz.PermFileAdmin)
	})
	c.FilesystemGrantPort = resourceauth.NewGrantService(repo, resourceAuthorizer, c.Transactor, c.EventBus, c.AuditLogger)
	storageRoot := c.Config.Filesystem.TransferGateway.StorageRoot
	ensureDefault := command.NewEnsureDefaultLocalProviderHandler(repo)
	createNamespace := command.NewCreateNamespaceHandler(repo, c.Transactor, c.EventBus, c.AuditLogger).
		WithDefaultLocalProvider(storageRoot, ensureDefault)
	if err := c.EventBus.Subscribe(
		integration.EventWorkspaceCreated,
		"filesystem.workspace-created.create-namespace",
		subscriber.NewOnWorkspaceCreated(createNamespace).Handle,
	); err != nil {
		return fmt.Errorf("subscribe filesystem workspace bootstrap: %w", err)
	}
	publicURL, secret, audience := transferGatewayDefaults(c)
	issuer, issuerErr := capability.NewIssuer(secret, audience)
	if issuerErr != nil {
		return fmt.Errorf("filesystem transfer capability issuer: %w", issuerErr)
	}
	createTransfer := command.NewCreateTransferHandler(repo, c.Transactor, c.EventBus, c.AuditLogger, nil)
	getTransfer := query.NewGetTransferHandler(repo)
	abortTransfer := command.NewAbortTransferHandler(repo, c.Transactor)
	handler := filesystemhttp.NewHandler(
		repo,
		query.NewListChildrenHandler(repo),
		query.NewListRevisionsHandler(repo),
		command.NewCreateDirectoryHandler(repo, c.Transactor, c.EventBus, c.AuditLogger),
		command.NewRenameNodeHandler(repo, c.Transactor, c.EventBus, c.AuditLogger),
		command.NewMoveNodeHandler(repo, c.Transactor, c.EventBus, c.AuditLogger),
		command.NewCopyNodeHandler(repo, c.Transactor, c.EventBus, c.AuditLogger),
		command.NewTrashNodeHandler(repo, c.Transactor, c.EventBus, c.AuditLogger),
		command.NewRestoreRevisionHandler(repo, c.Transactor, c.EventBus, c.AuditLogger),
		resourceAuthorizer,
		c.AuditLogger,
		&filesystemhttp.TransferOptions{
			Create:  createTransfer.Handle,
			Get:     getTransfer.Handle,
			Abort:   abortTransfer.Handle,
			Peek:    repo.GetTransferByID,
			Gateway: filesystemhttp.NewTransferGatewayView(publicURL, issuer),
		},
		&filesystemhttp.StorageAdminOptions{
			RegisterProvider:      command.NewRegisterProviderHandler(repo).Handle,
			ListStorageProviders:  query.NewListStorageProvidersHandler(repo).Handle,
			CreateStoragePool:     command.NewCreateStoragePoolHandler(repo).Handle,
			ListStoragePools:      query.NewListStoragePoolsHandler(repo).Handle,
			CreateStorageBinding:  command.NewCreateStorageBindingHandler(repo).Handle,
			ListStorageBindings:   query.NewListStorageBindingsHandler(repo).Handle,
			UpsertLifecyclePolicy: command.NewUpsertLifecyclePolicyHandler(repo).Handle,
			ListLifecyclePolicies: query.NewListLifecyclePoliciesHandler(repo).Handle,
		},
	)
	grantCommands := command.NewGrantResourceHandler(c.FilesystemGrantPort)
	agentSigner, agentSignerErr := agentsign.NewSigner(secret, domainservice.AgentCapabilityAudience)
	if agentSignerErr != nil {
		return fmt.Errorf("filesystem agent capability signer: %w", agentSignerErr)
	}
	accessStore := persistence.NewCapabilityRepository(c.DB)
	handler.BindArtifacts(filesystemhttp.ArtifactHandlers{
		Create: command.NewCreateArtifactHandler(repo, c.Transactor, c.EventBus).Handle,
		List:   query.NewListArtifactsHandler(repo).Handle,
	})
	previewReader := query.NewPreviewBlobReader(repo)
	searchIndex := query.NewRepositorySearchIndex(repo)
	handler.BindSearch(filesystemhttp.SearchHandlers{
		Search: query.NewSearchFilesHandler(
			searchIndex,
			nil,
			query.NewAuthorizationSearchAuthorizer(resourceAuthorizer),
		).Handle,
		GetPreviewBlob: previewReader.BlobID,
	})
	if auditService, ok := c.AuditLogger.(*audit.Service); ok {
		handler.BindAuditList(query.NewListFileAuditEventsHandler(auditService).Handle)
	}
	recordUploaded := command.NewRecordTransferUploadedHandler(repo, c.Transactor, c.EventBus)
	runtimeCatalog, err := fsstorage.NewRuntimeCatalog(repo, fsstorage.RuntimeCatalogConfig{LocalFallbackRoot: storageRoot})
	if err != nil {
		return fmt.Errorf("filesystem runtime catalog: %w", err)
	}
	gatewayControl := gatewaycontrol.NewService(repo, runtimeCatalog, recordUploaded.Handle)
	beginSFTP := command.NewBeginSFTPUploadHandler(repo, createTransfer, storageRoot, ensureDefault)
	filesystemhttp.RegisterInternalGatewayRoutes(c.Engine, c.Config, filesystemhttp.InternalGatewayHandlers{
		Control: gatewayControl,
		Token:   c.Config.Filesystem.TransferGateway.InternalToken,
	}, filesystemhttp.InternalSFTPHandlers{BeginSFTP: beginSFTP.Handle})
	handler.BindResourceAccess(filesystemhttp.ResourceAccess{
		Grant:            grantCommands.Handle,
		ListGrants:       grantCommands.List,
		RevokeGrant:      grantCommands.Revoke,
		IssueCapability:  command.NewIssueCapabilityHandler(domainservice.NewCapabilityIssuer(agentSigner), accessStore).Handle,
		RevokeCapability: command.NewRevokeCapabilityHandler(accessStore).Handle,
		StartExport:      command.NewStartControlledExportHandler(accessStore, c.AuditLogger).Handle,
		ImportExported:   operation.NewImportExportedFileHandler(repo, accessStore).Handle,
	})
	filesystemhttp.RegisterRoutes(
		c.Engine,
		entitlements.Middleware(c.DB),
		authn.RequireAuth(m.tokens),
		c.WorkspaceMemberChecker,
		c.PermissionChecker,
		handler,
	)
	return nil
}
