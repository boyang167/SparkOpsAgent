package http

import (
	"context"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/application/command"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/application/query"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/domain"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/httpx"
)

type ArtifactHandlers struct {
	Create func(context.Context, command.CreateArtifact) (model.Artifact, error)
	List   func(context.Context, query.ListArtifacts) ([]model.Artifact, string, error)
}

func (h *Handler) BindArtifacts(handlers ArtifactHandlers) {
	if h == nil {
		return
	}
	h.createArtifact = handlers.Create
	h.listArtifacts = handlers.List
}

type createArtifactBody struct {
	ArtifactType      string   `json:"artifact_type"`
	ProducerType      string   `json:"producer_type"`
	ProducerID        string   `json:"producer_id"`
	RunID             string   `json:"run_id"`
	InputRefs         []string `json:"input_refs"`
	FileID            string   `json:"file_id"`
	RetentionPolicyID string   `json:"retention_policy_id"`
}

func (h *Handler) CreateArtifact(c *gin.Context) {
	rc, ok := h.requestContext(c, true)
	if !ok {
		return
	}
	var body createArtifactBody
	if !decodeJSON(c, &body) {
		return
	}
	producerID, okProducer := canonicalUUID(body.ProducerID)
	runID, okRun := canonicalUUID(body.RunID)
	fileID, okFile := canonicalUUID(body.FileID)
	if !okProducer || !okRun || !okFile || strings.TrimSpace(body.ArtifactType) == "" || strings.TrimSpace(body.ProducerType) == "" {
		invalidRequest(c)
		return
	}
	inputRefs := make([]uuid.UUID, 0, len(body.InputRefs))
	for _, raw := range body.InputRefs {
		id, valid := canonicalUUID(raw)
		if !valid {
			invalidRequest(c)
			return
		}
		inputRefs = append(inputRefs, id)
	}
	var retention *uuid.UUID
	if strings.TrimSpace(body.RetentionPolicyID) != "" {
		id, valid := canonicalUUID(body.RetentionPolicyID)
		if !valid {
			invalidRequest(c)
			return
		}
		retention = &id
	}
	if h.createArtifact == nil {
		filesystemError(c, domain.ErrPersistence)
		return
	}
	auth := h.resourceDecision(c, rc, fileID, model.ResourcePermissionRead)
	if !auth.Allowed {
		filesystemError(c, domain.ErrResourceDenied)
		return
	}
	artifact, err := h.createArtifact(c.Request.Context(), command.CreateArtifact{
		CommandID: rc.commandID, ActorID: rc.actorID, OrganizationID: rc.namespace.OrganizationID,
		WorkspaceID: rc.workspaceID, ArtifactType: strings.TrimSpace(body.ArtifactType),
		ProducerType: strings.TrimSpace(body.ProducerType), ProducerID: producerID, RunID: runID,
		InputRefs: inputRefs, FileID: fileID, RetentionPolicyID: retention, TraceID: rc.traceID,
	})
	if err != nil {
		filesystemError(c, err)
		return
	}
	created(c, artifactView(artifact))
}

func (h *Handler) ListArtifacts(c *gin.Context) {
	rc, ok := h.requestContext(c, false)
	if !ok {
		return
	}
	var runID *uuid.UUID
	if raw := strings.TrimSpace(c.Query("run_id")); raw != "" {
		id, valid := canonicalUUID(raw)
		if !valid {
			invalidRequest(c)
			return
		}
		runID = &id
	}
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
	if h.listArtifacts == nil {
		filesystemError(c, domain.ErrPersistence)
		return
	}
	items, cursor, err := h.listArtifacts(c.Request.Context(), query.ListArtifacts{
		WorkspaceID: rc.workspaceID, RunID: runID, Limit: limit, Cursor: c.Query("cursor"),
	})
	if err != nil {
		filesystemError(c, err)
		return
	}
	views := make([]gin.H, 0, len(items))
	for _, item := range items {
		views = append(views, artifactView(item))
	}
	payload := gin.H{"items": views}
	if cursor != "" {
		payload["next_cursor"] = cursor
	}
	httpx.OK(c, payload)
}

func artifactView(artifact model.Artifact) gin.H {
	refs := make([]string, 0, len(artifact.InputRefs))
	for _, ref := range artifact.InputRefs {
		refs = append(refs, ref.String())
	}
	view := gin.H{
		"id": artifact.ID, "workspace_id": artifact.WorkspaceID, "namespace_id": artifact.NamespaceID,
		"artifact_type": artifact.ArtifactType, "producer_type": artifact.ProducerType,
		"producer_id": artifact.ProducerID, "run_id": artifact.RunID, "input_refs": refs,
		"file_id": artifact.FileID, "created_at": artifact.CreatedAt,
	}
	if artifact.RevisionID != nil {
		view["revision_id"] = *artifact.RevisionID
	}
	if artifact.RetentionPolicyID != nil {
		view["retention_policy_id"] = *artifact.RetentionPolicyID
	}
	return view
}
