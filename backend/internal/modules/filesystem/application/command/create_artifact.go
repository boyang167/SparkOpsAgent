package command

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/domain"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/domain/event"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/domain/repository"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/messaging"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/postgres"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/events"
)

type CreateArtifact struct {
	CommandID         uuid.UUID
	ActorID           uuid.UUID
	OrganizationID    uuid.UUID
	WorkspaceID       uuid.UUID
	ArtifactType      string
	ProducerType      string
	ProducerID        uuid.UUID
	RunID             uuid.UUID
	InputRefs         []uuid.UUID
	FileID            uuid.UUID
	RetentionPolicyID *uuid.UUID
	TraceID           string
}

type CreateArtifactHandler struct {
	repo     artifactRepo
	commands repository.FilesystemRepository
	tx       postgres.Transactor
	bus      messaging.Bus
}

type artifactRepo interface {
	GetNamespaceByWorkspace(context.Context, uuid.UUID) (*model.Namespace, error)
	GetNodeByID(context.Context, uuid.UUID) (*model.Node, error)
	CreateArtifact(context.Context, model.Artifact, model.FileLink) error
}

func NewCreateArtifactHandler(repo repository.FilesystemRepository, tx postgres.Transactor, bus messaging.Bus) *CreateArtifactHandler {
	requireMutationDependencies(repo, tx)
	return &CreateArtifactHandler{repo: repo, commands: repo, tx: tx, bus: bus}
}

func NewCreateArtifactHandlerWithRepo(repo artifactRepo, tx postgres.Transactor, bus messaging.Bus) *CreateArtifactHandler {
	return &CreateArtifactHandler{repo: repo, tx: tx, bus: bus}
}

func (h *CreateArtifactHandler) Handle(ctx context.Context, cmd CreateArtifact) (model.Artifact, error) {
	if h == nil || h.repo == nil {
		return model.Artifact{}, domain.ErrPersistence
	}
	ns, err := h.repo.GetNamespaceByWorkspace(ctx, cmd.WorkspaceID)
	if err != nil {
		return model.Artifact{}, err
	}
	if ns.OrganizationID != cmd.OrganizationID {
		return model.Artifact{}, domain.ErrNamespaceMismatch
	}
	node, err := h.repo.GetNodeByID(ctx, cmd.FileID)
	if err != nil {
		return model.Artifact{}, err
	}
	if node.NamespaceID != ns.ID {
		return model.Artifact{}, domain.ErrCrossWorkspaceReference
	}
	if node.Type != model.NodeTypeFile {
		return model.Artifact{}, domain.ErrUnsupportedNodeType
	}
	artifactID := uuid.New()
	if cmd.CommandID != uuid.Nil {
		artifactID = uuid.NewSHA1(uuid.NameSpaceURL, []byte("filesystem-artifact:"+cmd.CommandID.String()))
	}
	now := time.Now().UTC()
	inputRefs := append([]uuid.UUID{}, cmd.InputRefs...)
	artifact := model.Artifact{
		ID:                artifactID,
		WorkspaceID:       cmd.WorkspaceID,
		NamespaceID:       ns.ID,
		ArtifactType:      cmd.ArtifactType,
		ProducerType:      cmd.ProducerType,
		ProducerID:        cmd.ProducerID,
		RunID:             cmd.RunID,
		InputRefs:         inputRefs,
		FileID:            cmd.FileID,
		RevisionID:        node.CurrentRevisionID,
		RetentionPolicyID: cmd.RetentionPolicyID,
		CreatedAt:         now,
	}
	link := model.FileLink{
		ID:         uuid.New(),
		NodeID:     cmd.FileID,
		RevisionID: node.CurrentRevisionID,
		LinkType:   model.FileLinkTypeArtifact,
		LinkID:     artifactID,
		CreatedAt:  now,
	}
	if h.tx == nil {
		return model.Artifact{}, domain.ErrPersistence
	}
	err = withinIdempotentMutation(ctx, h.tx, func(txCtx context.Context) error {
		if h.commands != nil {
			requestHash := mutationRequestHash("create_artifact", cmd.WorkspaceID, cmd.OrganizationID, cmd.FileID, cmd.RunID, cmd.ArtifactType, cmd.ProducerType, cmd.ProducerID, inputRefs, cmd.RetentionPolicyID)
			completed, claimed, claimErr := claimMutation(txCtx, h.commands, cmd.CommandID, cmd.WorkspaceID, "create_artifact", requestHash, &artifact.ID, artifact.RevisionID)
			if claimErr != nil {
				return claimErr
			}
			if completed {
				if claimed == nil || claimed.NodeID == nil {
					return domain.ErrOutcomeUnknown
				}
				loaded, loadErr := h.commands.GetArtifact(txCtx, *claimed.NodeID)
				if loadErr != nil {
					return loadErr
				}
				artifact = loaded
				return nil
			}
		}
		if err := h.repo.CreateArtifact(txCtx, artifact, link); err != nil {
			return err
		}
		if h.bus != nil {
			if err := h.bus.Publish(txCtx, event.ArtifactCreated{
				BaseEvent:   events.NewBaseEvent(event.ArtifactCreatedName),
				ArtifactID:  artifact.ID,
				WorkspaceID: cmd.WorkspaceID,
				FileID:      cmd.FileID,
				RunID:       cmd.RunID,
			}); err != nil {
				return err
			}
		}
		if h.commands != nil {
			return completeMutation(txCtx, h.commands, cmd.CommandID, &artifact.ID, artifact.RevisionID)
		}
		return nil
	})
	if err != nil {
		return model.Artifact{}, err
	}
	return artifact, nil
}
