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
	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/domain"
	filesystemevent "github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/domain/event"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/domain/model"
	domainservice "github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/domain/service"
	agentsign "github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/infrastructure/capability"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/infrastructure/gatewaycontrol"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/infrastructure/persistence"
	fsstorage "github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/infrastructure/storage"
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
	tokens         *authn.TokenProvider
	runtimeCatalog *fsstorage.RuntimeCatalog
}

type workspaceAuthorityAdapter struct {
	resolver authz.WorkspaceAccessResolver
}

func (a workspaceAuthorityAdapter) ResolveWorkspaceAccess(ctx context.Context, workspaceID, actorID uuid.UUID) (uuid.UUID, bool, error) {
	organizationID, active, err := a.resolver.ResolveWorkspaceAccess(
		ctx, kernel.WorkspaceID{UUID: workspaceID}, kernel.MemberID{UUID: actorID},
	)
	return organizationID.UUID, active, err
}

type workspaceRoleReaderAdapter struct {
	reader authz.WorkspaceRoleReader
}

func (a workspaceRoleReaderAdapter) ListWorkspaceRoleIDs(ctx context.Context, workspaceID, actorID uuid.UUID) ([]uuid.UUID, error) {
	roleIDs, err := a.reader.ListWorkspaceRoleIDs(
		ctx, kernel.WorkspaceID{UUID: workspaceID}, kernel.MemberID{UUID: actorID},
	)
	if err != nil {
		return nil, err
	}
	ids := make([]uuid.UUID, 0, len(roleIDs))
	for _, roleID := range roleIDs {
		ids = append(ids, roleID.UUID)
	}
	return ids, nil
}

type namespaceAuthorityAdapter struct {
	workspaces      authz.WorkspaceAccessResolver
	rolePermissions authz.WorkspaceRolePermissionChecker
	organizations   authz.OrgPermissionChecker
}

type shareSourceAuthorizerAdapter struct {
	authorizer *resourceauth.Service
}

type adoptionAuthorizerAdapter struct {
	authorizer *resourceauth.Service
}

func (a adoptionAuthorizerAdapter) Authorize(ctx context.Context, request command.AdoptionAuthorizationRequest) bool {
	if a.authorizer == nil {
		return false
	}
	return a.authorizer.Authorize(ctx, resourceauth.Request{
		ActorID: request.ActorID, WorkspaceID: request.WorkspaceID,
		NamespaceID: request.NamespaceID, NodeID: request.NodeID, Permission: request.Permission,
	}).Allowed
}

func (a shareSourceAuthorizerAdapter) AuthorizeShare(ctx context.Context, actorID, workspaceID, namespaceID, nodeID uuid.UUID) (command.ShareAuthorization, error) {
	if a.authorizer == nil {
		return command.ShareAuthorization{}, nil
	}
	request := resourceauth.Request{
		ActorID: actorID, WorkspaceID: workspaceID, NamespaceID: namespaceID,
		NodeID: nodeID, Permission: model.ResourcePermissionShare,
	}
	decision := a.authorizer.Authorize(ctx, request)
	if !decision.Allowed {
		return command.ShareAuthorization{}, nil
	}
	independent, err := a.authorizer.HasIndependentShareAuthority(ctx, request)
	if err != nil {
		return command.ShareAuthorization{}, err
	}
	return command.ShareAuthorization{
		Allowed: true, RequiresReshareProof: !independent,
	}, nil
}

func (a shareSourceAuthorizerAdapter) VerifyReshareProof(ctx context.Context, actorID, workspaceID, namespaceID, nodeID uuid.UUID, sourceGrantIDs []uuid.UUID) error {
	if a.authorizer == nil {
		return domain.ErrResourceDenied
	}
	return a.authorizer.VerifyReshareProof(ctx, resourceauth.ReshareProofRequest{
		ActorID: actorID, WorkspaceID: workspaceID, NamespaceID: namespaceID,
		NodeID: nodeID, SourceGrantIDs: sourceGrantIDs,
	})
}

type principalScopeAdapter struct {
	workspaces    authz.WorkspaceScopeResolver
	roles         authz.RoleScopeResolver
	organizations authz.OrgPermissionChecker
}

func (a principalScopeAdapter) Resolve(ctx context.Context, principalType model.PrincipalType, principalID, expectedOrganizationID uuid.UUID) (command.PrincipalScope, error) {
	scope := command.PrincipalScope{OrganizationID: expectedOrganizationID}
	switch principalType {
	case model.PrincipalUser:
		member, err := a.organizations.IsOrgMember(ctx, kernel.OrgID{UUID: expectedOrganizationID}, kernel.MemberID{UUID: principalID})
		if err != nil {
			return command.PrincipalScope{}, err
		}
		if !member {
			return command.PrincipalScope{}, domain.ErrCrossOrganizationGrant
		}
		return scope, nil
	case model.PrincipalOrganization:
		if principalID != expectedOrganizationID {
			return command.PrincipalScope{}, domain.ErrCrossOrganizationGrant
		}
		return scope, nil
	case model.PrincipalWorkspace:
		return a.resolveWorkspace(ctx, principalID, expectedOrganizationID)
	case model.PrincipalRole:
		workspaceID, err := a.roles.ResolveRoleWorkspace(ctx, kernel.RoleID{UUID: principalID})
		if err != nil {
			return command.PrincipalScope{}, err
		}
		return a.resolveWorkspace(ctx, workspaceID.UUID, expectedOrganizationID)
	default:
		return command.PrincipalScope{}, domain.ErrUnsupportedPrincipal
	}
}

func (a principalScopeAdapter) resolveWorkspace(ctx context.Context, workspaceID, expectedOrganizationID uuid.UUID) (command.PrincipalScope, error) {
	organizationID, active, err := a.workspaces.ResolveWorkspaceOrganization(ctx, kernel.WorkspaceID{UUID: workspaceID})
	if err != nil {
		return command.PrincipalScope{}, err
	}
	if !active || organizationID.UUID != expectedOrganizationID {
		return command.PrincipalScope{}, domain.ErrCrossOrganizationGrant
	}
	return command.PrincipalScope{OrganizationID: organizationID.UUID, WorkspaceID: workspaceID}, nil
}

func (a namespaceAuthorityAdapter) HasWorkspaceFileAdmin(ctx context.Context, workspaceID, actorID uuid.UUID) (bool, error) {
	_, active, err := a.workspaces.ResolveWorkspaceAccess(
		ctx, kernel.WorkspaceID{UUID: workspaceID}, kernel.MemberID{UUID: actorID},
	)
	if err != nil || !active {
		return false, err
	}
	return a.rolePermissions.HasWorkspaceRolePermission(
		ctx, kernel.WorkspaceID{UUID: workspaceID}, kernel.MemberID{UUID: actorID}, authz.PermFileAdmin,
	)
}

func (a namespaceAuthorityAdapter) HasOrganizationFileAdmin(ctx context.Context, organizationID, actorID uuid.UUID) (bool, error) {
	orgID, memberID := kernel.OrgID{UUID: organizationID}, kernel.MemberID{UUID: actorID}
	member, err := a.organizations.IsOrgMember(ctx, orgID, memberID)
	if err != nil || !member {
		return false, err
	}
	return a.organizations.HasOrgPermission(ctx, orgID, memberID, authz.PermOrgFileAdmin)
}

func New(tokens *authn.TokenProvider) *Module {
	return &Module{tokens: tokens}
}

func (m *Module) Name() string { return "filesystem" }

func (m *Module) Close() error {
	if m == nil || m.runtimeCatalog == nil {
		return nil
	}
	catalog := m.runtimeCatalog
	m.runtimeCatalog = nil
	return catalog.Close()
}

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
	if c.WorkspaceAccessResolver == nil {
		return fmt.Errorf("filesystem module requires workspace access resolver")
	}
	if c.WorkspaceScopeResolver == nil {
		return fmt.Errorf("filesystem module requires workspace scope resolver")
	}
	if c.WorkspaceRoleReader == nil {
		return fmt.Errorf("filesystem module requires workspace role reader")
	}
	if c.WorkspaceRolePermissions == nil {
		return fmt.Errorf("filesystem module requires workspace role permission checker")
	}
	if c.RoleScopeResolver == nil {
		return fmt.Errorf("filesystem module requires role scope resolver")
	}
	if c.OrgPermissionChecker == nil {
		return fmt.Errorf("filesystem module requires organization permission checker")
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
	principalResolver := resourceauth.NewPrincipalResolver(
		workspaceAuthorityAdapter{resolver: c.WorkspaceAccessResolver},
		workspaceRoleReaderAdapter{reader: c.WorkspaceRoleReader},
	)
	resourceAuthorizer := resourceauth.NewService(repo, principalResolver, namespaceAuthorityAdapter{
		workspaces: c.WorkspaceAccessResolver, rolePermissions: c.WorkspaceRolePermissions, organizations: c.OrgPermissionChecker,
	})
	c.FilesystemGrantPort = resourceauth.NewGrantService(repo, resourceAuthorizer, c.Transactor, c.EventBus, c.AuditLogger)
	storageRoot := c.Config.Filesystem.TransferGateway.StorageRoot
	ensureDefault := command.NewEnsureDefaultLocalProviderHandler(repo)
	ensureNamespace := command.NewEnsureNamespaceHandler(repo, c.Transactor).
		WithDefaultLocalProvider(storageRoot, ensureDefault)
	if err := c.EventBus.Subscribe(
		integration.EventWorkspaceCreated,
		"filesystem.workspace-created.create-namespace",
		subscriber.NewOnWorkspaceCreated(ensureNamespace).Handle,
	); err != nil {
		return fmt.Errorf("subscribe filesystem workspace bootstrap: %w", err)
	}
	publicURL, secret, audience := transferGatewayDefaults(c)
	sharedCursorCodec, cursorErr := query.NewSharedCursorCodec(secret, nil)
	if cursorErr != nil {
		return fmt.Errorf("filesystem shared cursor codec: %w", cursorErr)
	}
	issuer, issuerErr := capability.NewIssuer(secret, audience)
	if issuerErr != nil {
		return fmt.Errorf("filesystem transfer capability issuer: %w", issuerErr)
	}
	createTransfer := command.NewCreateTransferHandler(
		repo, c.Transactor, c.EventBus, c.AuditLogger, domainservice.NewRepositoryPlacement(repo),
	).
		WithDirectLocalProvider(storageRoot)
	getTransfer := query.NewGetTransferHandler(repo)
	listTransfers := query.NewListTransfersHandler(repo)
	abortTransfer := command.NewAbortTransferHandler(repo, c.Transactor)
	recordUploaded := command.NewRecordTransferUploadedHandler(repo, c.Transactor, c.EventBus)
	runtimeCatalog, err := fsstorage.NewRuntimeCatalog(repo, fsstorage.RuntimeCatalogConfig{LocalFallbackRoot: storageRoot})
	if err != nil {
		return fmt.Errorf("filesystem runtime catalog: %w", err)
	}
	prepareMultipart := command.NewPrepareTransferMultipartHandler(repo, runtimeCatalog)
	completeMultipart := command.NewCompleteTransferMultipartHandler(repo, runtimeCatalog, recordUploaded)
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
			Create:            createTransfer.Handle,
			Get:               getTransfer.Handle,
			List:              listTransfers.Handle,
			Abort:             abortTransfer.Handle,
			CompleteMultipart: completeMultipart.Handle,
			PrepareMultipart:  prepareMultipart.Handle,
			Peek:              repo.GetTransferByID,
			Gateway:           filesystemhttp.NewTransferGatewayView(publicURL, issuer),
			ProviderCatalog:   runtimeCatalog,
			ProviderUploadID:  repo.TransferProviderUploadID,
		},
		&filesystemhttp.StorageAdminOptions{
			RegisterProvider:      command.NewRegisterProviderHandler(repo).Handle,
			UpdateProvider:        command.NewUpdateProviderHandler(repo).Handle,
			DeleteProvider:        command.NewDeleteProviderHandler(repo).Handle,
			ListStorageProviders:  query.NewListStorageProvidersHandler(repo).Handle,
			CreateStoragePool:     command.NewCreateStoragePoolHandler(repo).Handle,
			UpdateStoragePool:     command.NewUpdateStoragePoolHandler(repo).Handle,
			DeleteStoragePool:     command.NewDeleteStoragePoolHandler(repo).Handle,
			ListStoragePools:      query.NewListStoragePoolsHandler(repo).Handle,
			CreateStorageBinding:  command.NewCreateStorageBindingHandler(repo).Handle,
			ListStorageBindings:   query.NewListStorageBindingsHandler(repo).Handle,
			UpsertLifecyclePolicy: command.NewUpsertLifecyclePolicyHandler(repo).Handle,
			ListLifecyclePolicies: query.NewListLifecyclePoliciesHandler(repo).Handle,
		},
	)
	grantCommands := command.NewGrantResourceHandler(c.FilesystemGrantPort)
	shareAuthority := namespaceAuthorityAdapter{
		workspaces: c.WorkspaceAccessResolver, rolePermissions: c.WorkspaceRolePermissions, organizations: c.OrgPermissionChecker,
	}
	shareCommands := command.NewShareRequestHandler(
		repo, c.Transactor, c.EventBus,
		principalScopeAdapter{workspaces: c.WorkspaceScopeResolver, roles: c.RoleScopeResolver, organizations: c.OrgPermissionChecker},
		shareAuthority,
		shareSourceAuthorizerAdapter{authorizer: resourceAuthorizer},
	).WithAudit(c.AuditLogger)
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
	handler.BindFileStorage(query.NewGetFileStorageHandler(repo).Handle)
	var listFileAudits func(context.Context, query.ListFileAuditEvents) (query.ListFileAuditEventsResult, error)
	if auditService, ok := c.AuditLogger.(*audit.Service); ok {
		listFileAudits = query.NewListFileAuditEventsHandler(auditService).Handle
		handler.BindAuditList(listFileAudits)
	}
	m.runtimeCatalog = runtimeCatalog
	gatewayControl := gatewaycontrol.NewService(repo, runtimeCatalog, recordUploaded.Handle)
	beginSFTP := command.NewBeginSFTPUploadHandler(repo, createTransfer, storageRoot, ensureDefault)
	filesystemhttp.RegisterInternalGatewayRoutes(c.Engine, c.Config, filesystemhttp.InternalGatewayHandlers{
		Control: gatewayControl,
		Token:   c.Config.Filesystem.TransferGateway.InternalToken,
	}, filesystemhttp.InternalSFTPHandlers{BeginSFTP: beginSFTP.Handle})
	handler.BindResourceAccess(filesystemhttp.ResourceAccess{
		Grant:            grantCommands.Handle,
		Share:            shareCommands.Submit,
		ListGrants:       grantCommands.List,
		RevokeGrant:      grantCommands.Revoke,
		IssueCapability:  command.NewIssueCapabilityHandler(domainservice.NewCapabilityIssuer(agentSigner), accessStore).Handle,
		RevokeCapability: command.NewRevokeCapabilityHandler(accessStore).Handle,
		StartExport:      command.NewStartControlledExportHandler(accessStore, c.AuditLogger, repo).Handle,
		ImportExported:   operation.NewImportExportedFileHandler(repo, accessStore).Handle,
	})
	entitlementMiddleware := entitlements.Middleware(c.DB)
	governanceHandler := filesystemhttp.NewGovernanceHandler(
		repo,
		query.NewListNamespacesHandler(repo, principalResolver, resourceAuthorizer).
			WithScopeEnsurer(ensureNamespace),
		query.NewSharedWithMeHandler(repo, principalResolver, resourceAuthorizer, sharedCursorCodec),
		query.NewEffectiveAccessHandler(resourceAuthorizer),
		resourceAuthorizer,
		sharedCursorCodec,
	)
	governanceHandler.BindShareWorkflow(shareCommands.Preview, shareCommands.Submit, shareCommands.Approve, shareCommands.Reject)
	governanceHandler.BindAdoption(
		command.NewAdoptFileHandler(repo, c.Transactor, adoptionAuthorizerAdapter{authorizer: resourceAuthorizer}, c.AuditLogger).
			WithBus(c.EventBus).Handle,
	)
	if listFileAudits != nil {
		governanceHandler.BindAuditList(listFileAudits)
	}
	governanceHandler.BindGrantRevocation(grantCommands.Revoke)
	filesystemhttp.RegisterGovernanceRoutes(
		c.Engine,
		entitlementMiddleware,
		authn.RequireAuth(m.tokens),
		c.OrgPermissionChecker,
		c.WorkspaceAccessResolver,
		governanceHandler,
	)
	filesystemhttp.RegisterRoutes(
		c.Engine,
		entitlementMiddleware,
		authn.RequireAuth(m.tokens),
		c.WorkspaceMemberChecker,
		c.PermissionChecker,
		handler,
	)
	return nil
}
