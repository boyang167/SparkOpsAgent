package application

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	fscommand "github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/application/command"
	fsquery "github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/application/query"
	fsdomain "github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/domain"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/domain/repository"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/domain/service"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/mcp/domain"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/authz"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

const (
	actionFileRead  = "file:read"
	actionFileWrite = "file:write"
)

type capabilityVerifier interface {
	Verify(token string, now time.Time) (service.Claims, error)
}

type FilesystemToolDeps struct {
	CapabilityStore repository.CapabilityRepository
	Verifier        capabilityVerifier
	GetNamespace    func(context.Context, uuid.UUID) (*model.Namespace, error)
	ListChildren    *fsquery.ListChildrenHandler
	GetNode         func(context.Context, uuid.UUID) (*model.Node, error)
	GetRevision     func(context.Context, uuid.UUID, uuid.UUID) (*model.FileRevision, error)
	CreateTransfer  func(context.Context, fscommand.CreateTransfer) (fscommand.TransferDescriptor, error)
	CreateArtifact  func(context.Context, fscommand.CreateArtifact) (model.Artifact, error)
}

func RegisterFilesystemTools(registry *domain.Registry, deps FilesystemToolDeps) {
	if registry == nil {
		return
	}
	moduleKey := "filesystem"
	sharedProps := map[string]any{
		"capability_token": domain.StringProperty("Run-scoped filesystem capability JWT"),
	}

	registry.Register(domain.ToolSpec{
		Name:        "list_files",
		Description: "List files and directories in the workspace namespace (capability scoped).",
		InputSchema: domain.ObjectSchema(mergeProps(sharedProps, map[string]any{
			"parent_id": domain.StringProperty("Optional parent directory UUID"),
			"limit":     domain.IntegerProperty("Page size"),
			"cursor":    domain.StringProperty("Pagination cursor"),
		}), "capability_token"),
		Permission: authz.PermFileRead,
		Modules:    []string{moduleKey},
		Tier:       domain.ToolTierRead,
	}, filesystemHandler(deps, actionFileRead, listFiles))

	registry.Register(domain.ToolSpec{
		Name:        "get_file_metadata",
		Description: "Get metadata for a file node (capability scoped).",
		InputSchema: domain.ObjectSchema(mergeProps(sharedProps, map[string]any{
			"file_id": domain.StringProperty("File node UUID"),
		}), "capability_token", "file_id"),
		Permission: authz.PermFileRead,
		Modules:    []string{moduleKey},
		Tier:       domain.ToolTierRead,
	}, filesystemHandler(deps, actionFileRead, getFileMetadata))

	registry.Register(domain.ToolSpec{
		Name:        "read_file",
		Description: "Read current revision metadata for a file (capability scoped; bytes via transfer gateway).",
		InputSchema: domain.ObjectSchema(mergeProps(sharedProps, map[string]any{
			"file_id": domain.StringProperty("File node UUID"),
		}), "capability_token", "file_id"),
		Permission: authz.PermFileRead,
		Modules:    []string{moduleKey},
		Tier:       domain.ToolTierRead,
	}, filesystemHandler(deps, actionFileRead, readFile))

	registry.Register(domain.ToolSpec{
		Name:        "create_upload_session",
		Description: "Create an upload transfer session for a new or existing file path (capability scoped).",
		InputSchema: domain.ObjectSchema(mergeProps(sharedProps, map[string]any{
			"parent_id": domain.StringProperty("Parent directory UUID"),
			"name":      domain.StringProperty("File name"),
			"size":      domain.IntegerProperty("Expected file size in bytes"),
			"sha256":    domain.StringProperty("Optional SHA-256 hex digest"),
		}), "capability_token", "parent_id", "name", "size"),
		Permission: authz.PermFileWrite,
		Modules:    []string{moduleKey},
		Tier:       domain.ToolTierAction,
	}, filesystemHandler(deps, actionFileWrite, createUploadSession))

	registry.Register(domain.ToolSpec{
		Name:        "write_file",
		Description: "Alias for create_upload_session when replacing file bytes.",
		InputSchema: domain.ObjectSchema(mergeProps(sharedProps, map[string]any{
			"file_id": domain.StringProperty("Existing file node UUID"),
			"size":    domain.IntegerProperty("Expected file size in bytes"),
			"sha256":  domain.StringProperty("Optional SHA-256 hex digest"),
		}), "capability_token", "file_id", "size"),
		Permission: authz.PermFileWrite,
		Modules:    []string{moduleKey},
		Tier:       domain.ToolTierAction,
	}, filesystemHandler(deps, actionFileWrite, writeFile))

	registry.Register(domain.ToolSpec{
		Name:        "create_artifact",
		Description: "Register an artifact linked to a run and file (capability scoped).",
		InputSchema: domain.ObjectSchema(mergeProps(sharedProps, map[string]any{
			"file_id":       domain.StringProperty("File node UUID"),
			"run_id":        domain.StringProperty("Agent or workflow run UUID"),
			"artifact_type": domain.StringProperty("Artifact type"),
			"producer_type": domain.StringProperty("Producer type"),
			"producer_id":   domain.StringProperty("Producer UUID"),
		}), "capability_token", "file_id", "run_id", "artifact_type", "producer_type", "producer_id"),
		Permission: authz.PermFileWrite,
		Modules:    []string{moduleKey},
		Tier:       domain.ToolTierAction,
	}, filesystemHandler(deps, actionFileWrite, createArtifact))
}

type scopedFilesystemCall func(context.Context, FilesystemToolDeps, service.Claims, kernel.WorkspaceID, kernel.MemberID, map[string]any) (any, error)

func filesystemHandler(deps FilesystemToolDeps, action string, call scopedFilesystemCall) domain.ToolHandler {
	return func(ctx context.Context, wsID kernel.WorkspaceID, memberID kernel.MemberID, args map[string]any) (any, error) {
		claims, err := resolveFilesystemCapability(ctx, deps, wsID, args)
		if err != nil {
			return nil, err
		}
		if err := requireCapabilityAction(claims, action); err != nil {
			return nil, err
		}
		return call(ctx, deps, claims, wsID, memberID, args)
	}
}

func resolveFilesystemCapability(
	ctx context.Context,
	deps FilesystemToolDeps,
	wsID kernel.WorkspaceID,
	args map[string]any,
) (service.Claims, error) {
	token, err := parseStringArg(args, "capability_token")
	if err != nil {
		return service.Claims{}, fsdomain.ErrCapabilityInvalid
	}
	if deps.Verifier == nil || deps.CapabilityStore == nil {
		return service.Claims{}, fsdomain.ErrCapabilityInvalid
	}
	now := time.Now().UTC()
	if err := fscommand.AuthorizeCapability(ctx, deps.Verifier, deps.CapabilityStore, token, now); err != nil {
		return service.Claims{}, err
	}
	claims, err := deps.Verifier.Verify(token, now)
	if err != nil {
		return service.Claims{}, err
	}
	if claims.WorkspaceID != wsID.UUID {
		return service.Claims{}, fsdomain.ErrCapabilityScopeDenied
	}
	return claims, nil
}

func requireCapabilityResource(claims service.Claims, resourceID uuid.UUID) error {
	for _, id := range claims.ResourceIDs {
		if id == resourceID {
			return nil
		}
	}
	return fsdomain.ErrCapabilityScopeDenied
}

func requireCapabilityAction(claims service.Claims, action string) error {
	if action == "" {
		return nil
	}
	for _, allowed := range claims.Actions {
		if allowed == action {
			return nil
		}
	}
	return fsdomain.ErrCapabilityScopeDenied
}

type readFileTool struct {
	claims service.Claims
	call   scopedFilesystemCall
	deps   FilesystemToolDeps
}

func newReadFileTool(claims service.Claims, deps FilesystemToolDeps) *readFileTool {
	return &readFileTool{
		claims: claims,
		deps:   deps,
		call:   readFile,
	}
}

func (t *readFileTool) Call(ctx context.Context, args map[string]any) (any, error) {
	fileIDRaw, err := parseStringArg(args, "file_id")
	if err != nil {
		return nil, err
	}
	fileID, err := uuid.Parse(fileIDRaw)
	if err != nil {
		return nil, fmt.Errorf("invalid file_id")
	}
	if err := requireCapabilityResource(t.claims, fileID); err != nil {
		return nil, err
	}
	if err := requireCapabilityAction(t.claims, actionFileRead); err != nil {
		return nil, err
	}
	ws := kernel.WorkspaceID{UUID: t.claims.WorkspaceID}
	member := kernel.MemberID{UUID: t.claims.ActorID}
	return t.call(ctx, t.deps, t.claims, ws, member, args)
}

func listFiles(ctx context.Context, deps FilesystemToolDeps, claims service.Claims, wsID kernel.WorkspaceID, _ kernel.MemberID, args map[string]any) (any, error) {
	if deps.GetNamespace == nil || deps.ListChildren == nil {
		return nil, fmt.Errorf("list_files unavailable")
	}
	ns, err := deps.GetNamespace(ctx, wsID.UUID)
	if err != nil {
		return nil, err
	}
	var parentID *uuid.UUID
	if raw, ok := args["parent_id"].(string); ok && raw != "" {
		id, parseErr := uuid.Parse(raw)
		if parseErr != nil {
			return nil, fmt.Errorf("invalid parent_id")
		}
		if err := requireCapabilityResource(claims, id); err != nil {
			return nil, err
		}
		parentID = &id
	}
	limit := parseIntArg(args, "limit", 50)
	cursor, _ := args["cursor"].(string)
	nodes, next, err := deps.ListChildren.Handle(ctx, fsquery.ListChildren{
		NamespaceID: ns.ID,
		ParentID:    parentID,
		Limit:       limit,
		Cursor:      cursor,
	})
	if err != nil {
		return nil, err
	}
	filtered := filterNodesByCapability(nodes, claims.ResourceIDs, parentID != nil)
	items := make([]map[string]any, 0, len(filtered))
	for _, node := range filtered {
		items = append(items, nodeView(node))
	}
	return map[string]any{"items": items, "next_cursor": next}, nil
}

func getFileMetadata(ctx context.Context, deps FilesystemToolDeps, claims service.Claims, _ kernel.WorkspaceID, _ kernel.MemberID, args map[string]any) (any, error) {
	node, err := loadScopedNode(ctx, deps, claims, args)
	if err != nil {
		return nil, err
	}
	return map[string]any{"file": nodeView(*node)}, nil
}

func readFile(ctx context.Context, deps FilesystemToolDeps, claims service.Claims, _ kernel.WorkspaceID, _ kernel.MemberID, args map[string]any) (any, error) {
	node, err := loadScopedNode(ctx, deps, claims, args)
	if err != nil {
		return nil, err
	}
	out := map[string]any{"file": nodeView(*node)}
	if node.CurrentRevisionID == nil || deps.GetRevision == nil {
		return out, nil
	}
	rev, err := deps.GetRevision(ctx, node.ID, *node.CurrentRevisionID)
	if err != nil {
		return nil, err
	}
	out["revision"] = map[string]any{
		"id":          rev.ID().String(),
		"size":        rev.Size(),
		"mime_type":   rev.MIMEType(),
		"revision_no": rev.RevisionNo(),
	}
	return out, nil
}

func createUploadSession(ctx context.Context, deps FilesystemToolDeps, claims service.Claims, wsID kernel.WorkspaceID, memberID kernel.MemberID, args map[string]any) (any, error) {
	if deps.CreateTransfer == nil || deps.GetNamespace == nil {
		return nil, fmt.Errorf("create_upload_session unavailable")
	}
	parentRaw, err := parseStringArg(args, "parent_id")
	if err != nil {
		return nil, err
	}
	parentID, err := uuid.Parse(parentRaw)
	if err != nil {
		return nil, fmt.Errorf("invalid parent_id")
	}
	if err := requireCapabilityResource(claims, parentID); err != nil {
		return nil, err
	}
	name, err := parseStringArg(args, "name")
	if err != nil {
		return nil, err
	}
	size := int64(parseIntArg(args, "size", 0))
	if size <= 0 {
		return nil, fmt.Errorf("missing required argument: size")
	}
	sha256, _ := args["sha256"].(string)
	ns, err := deps.GetNamespace(ctx, wsID.UUID)
	if err != nil {
		return nil, err
	}
	desc, err := deps.CreateTransfer(ctx, fscommand.CreateTransfer{
		ActorID:        memberID.UUID,
		OrganizationID: ns.OrganizationID,
		WorkspaceID:    wsID.UUID,
		NamespaceID:    ns.ID,
		ParentID:       &parentID,
		Name:           name,
		Protocol:       model.TransferProtocolMultipart,
		Size:           size,
		SHA256:         sha256,
		Classification: model.TransferClassificationInternal,
		ExpiresAt:      time.Now().UTC().Add(24 * time.Hour),
	})
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"session_id":  desc.SessionID.String(),
		"protocol":    desc.Protocol,
		"expires_at":  desc.ExpiresAt,
		"part_size":   desc.PartSize,
		"total_parts": desc.TotalParts,
	}, nil
}

func writeFile(ctx context.Context, deps FilesystemToolDeps, claims service.Claims, wsID kernel.WorkspaceID, memberID kernel.MemberID, args map[string]any) (any, error) {
	node, err := loadScopedNode(ctx, deps, claims, args)
	if err != nil {
		return nil, err
	}
	parentID := node.ID
	if node.ParentID != nil {
		parentID = *node.ParentID
	}
	return createUploadSession(ctx, deps, claims, wsID, memberID, map[string]any{
		"parent_id": parentID.String(),
		"name":      node.Name,
		"size":      args["size"],
		"sha256":    args["sha256"],
	})
}

func createArtifact(ctx context.Context, deps FilesystemToolDeps, claims service.Claims, wsID kernel.WorkspaceID, memberID kernel.MemberID, args map[string]any) (any, error) {
	if deps.CreateArtifact == nil || deps.GetNamespace == nil {
		return nil, fmt.Errorf("create_artifact unavailable")
	}
	node, err := loadScopedNode(ctx, deps, claims, args)
	if err != nil {
		return nil, err
	}
	runRaw, err := parseStringArg(args, "run_id")
	if err != nil {
		return nil, err
	}
	runID, err := uuid.Parse(runRaw)
	if err != nil {
		return nil, fmt.Errorf("invalid run_id")
	}
	if claims.RunID != uuid.Nil && claims.RunID != runID {
		return nil, fsdomain.ErrCapabilityScopeDenied
	}
	artifactType, err := parseStringArg(args, "artifact_type")
	if err != nil {
		return nil, err
	}
	producerType, err := parseStringArg(args, "producer_type")
	if err != nil {
		return nil, err
	}
	producerRaw, err := parseStringArg(args, "producer_id")
	if err != nil {
		return nil, err
	}
	producerID, err := uuid.Parse(producerRaw)
	if err != nil {
		return nil, fmt.Errorf("invalid producer_id")
	}
	ns, err := deps.GetNamespace(ctx, wsID.UUID)
	if err != nil {
		return nil, err
	}
	artifact, err := deps.CreateArtifact(ctx, fscommand.CreateArtifact{
		ActorID:        memberID.UUID,
		OrganizationID: ns.OrganizationID,
		WorkspaceID:    wsID.UUID,
		ArtifactType:   artifactType,
		ProducerType:   producerType,
		ProducerID:     producerID,
		RunID:          runID,
		FileID:         node.ID,
	})
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"artifact_id": artifact.ID.String(),
		"file_id":     artifact.FileID.String(),
		"run_id":      artifact.RunID.String(),
	}, nil
}

func loadScopedNode(ctx context.Context, deps FilesystemToolDeps, claims service.Claims, args map[string]any) (*model.Node, error) {
	if deps.GetNode == nil {
		return nil, fmt.Errorf("filesystem node lookup unavailable")
	}
	fileIDRaw, err := parseStringArg(args, "file_id")
	if err != nil {
		return nil, err
	}
	fileID, err := uuid.Parse(fileIDRaw)
	if err != nil {
		return nil, fmt.Errorf("invalid file_id")
	}
	if err := requireCapabilityResource(claims, fileID); err != nil {
		return nil, err
	}
	node, err := deps.GetNode(ctx, fileID)
	if err != nil {
		return nil, err
	}
	return node, nil
}

func filterNodesByCapability(nodes []model.Node, allowed []uuid.UUID, parentScoped bool) []model.Node {
	if len(allowed) == 0 {
		return nodes
	}
	allowedSet := make(map[uuid.UUID]struct{}, len(allowed))
	for _, id := range allowed {
		allowedSet[id] = struct{}{}
	}
	out := make([]model.Node, 0, len(nodes))
	for _, node := range nodes {
		if parentScoped {
			out = append(out, node)
			continue
		}
		if _, ok := allowedSet[node.ID]; ok {
			out = append(out, node)
		}
	}
	return out
}

func nodeView(node model.Node) map[string]any {
	view := map[string]any{
		"id":            node.ID.String(),
		"namespace_id":  node.NamespaceID.String(),
		"type":          string(node.Type),
		"name":          node.Name,
		"state":         string(node.State),
		"owner_id":      node.OwnerID.String(),
		"parent_id":     nil,
		"revision_id":   nil,
	}
	if node.ParentID != nil {
		view["parent_id"] = node.ParentID.String()
	}
	if node.CurrentRevisionID != nil {
		view["revision_id"] = node.CurrentRevisionID.String()
	}
	return view
}

func mergeProps(base, extra map[string]any) map[string]any {
	out := make(map[string]any, len(base)+len(extra))
	for k, v := range base {
		out[k] = v
	}
	for k, v := range extra {
		out[k] = v
	}
	return out
}
