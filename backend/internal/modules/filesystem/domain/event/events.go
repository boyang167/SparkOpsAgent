package event

import (
	"github.com/google/uuid"

	"github.com/sparkops/sparkops-agent/backend/internal/shared/events"
)

const (
	FileCreatedName         = "filesystem.file.created.v1"
	FileRevisionCreatedName = "filesystem.file.revision-created.v1"
	DirectoryCreatedName    = "filesystem.directory.created.v1"
	NodeRenamedName         = "filesystem.node.renamed.v1"
	NodeMovedName           = "filesystem.file.moved.v1"
	NodeCopiedName          = "filesystem.node.copied.v1"
	NodeTrashedName         = "filesystem.file.trashed.v1"
	NodeRestoredName        = "filesystem.node-restored.v1"
	NodePurgedName          = "filesystem.node-purged.v1"
	RevisionRestoredName    = "filesystem.revision.restored.v1"
	PermissionChangedName   = "filesystem.file.permission-changed.v1"
	TransferInitiatedName   = "filesystem.transfer.initiated.v1"
	TransferUploadedName    = "filesystem.transfer.uploaded.v1"
	TransferCompletedName   = "filesystem.transfer.completed.v1"
	ArtifactCreatedName     = "filesystem.artifact.created.v1"
)

type FileCreated struct {
	events.BaseEvent
	FileID      uuid.UUID `json:"file_id"`
	NamespaceID uuid.UUID `json:"namespace_id"`
}

type FileRevisionCreated struct {
	events.BaseEvent
	FileID     uuid.UUID `json:"file_id"`
	RevisionID uuid.UUID `json:"revision_id"`
	RevisionNo int       `json:"revision_no"`
}

type DirectoryCreated struct {
	events.BaseEvent
	NodeID      uuid.UUID `json:"node_id"`
	NamespaceID uuid.UUID `json:"namespace_id"`
	ParentID    uuid.UUID `json:"parent_id"`
}

type NodeRenamed struct {
	events.BaseEvent
	NodeID uuid.UUID `json:"node_id"`
}

type NodeMoved struct {
	events.BaseEvent
	NodeID   uuid.UUID `json:"node_id"`
	ParentID uuid.UUID `json:"parent_id"`
}

type NodeCopied struct {
	events.BaseEvent
	SourceNodeID uuid.UUID `json:"source_node_id"`
	NodeID       uuid.UUID `json:"node_id"`
	RevisionID   uuid.UUID `json:"revision_id"`
}

type NodeTrashed struct {
	events.BaseEvent
	NodeID uuid.UUID `json:"node_id"`
}

type NodeRestored struct {
	events.BaseEvent
	NodeID uuid.UUID `json:"node_id"`
}

type NodePurged struct {
	events.BaseEvent
	NodeID uuid.UUID `json:"node_id"`
}

type RevisionRestored struct {
	events.BaseEvent
	NodeID     uuid.UUID `json:"node_id"`
	RevisionID uuid.UUID `json:"revision_id"`
}

type PermissionChanged struct {
	events.BaseEvent
	NodeID  uuid.UUID `json:"node_id"`
	GrantID uuid.UUID `json:"grant_id"`
	Action  string    `json:"action"`
}

type ArtifactCreated struct {
	events.BaseEvent
	ArtifactID  uuid.UUID `json:"artifact_id"`
	WorkspaceID uuid.UUID `json:"workspace_id"`
	FileID      uuid.UUID `json:"file_id"`
	RunID       uuid.UUID `json:"run_id"`
}

type TransferInitiated struct {
	events.BaseEvent
	SessionID   uuid.UUID `json:"session_id"`
	WorkspaceID uuid.UUID `json:"workspace_id"`
	NamespaceID uuid.UUID `json:"namespace_id"`
}

type TransferUploaded struct {
	events.BaseEvent
	SessionID   uuid.UUID `json:"session_id"`
	WorkspaceID uuid.UUID `json:"workspace_id"`
	NamespaceID uuid.UUID `json:"namespace_id"`
}

type TransferCompleted struct {
	events.BaseEvent
	SessionID   uuid.UUID `json:"session_id"`
	WorkspaceID uuid.UUID `json:"workspace_id"`
	NamespaceID uuid.UUID `json:"namespace_id"`
	NodeID      uuid.UUID `json:"node_id"`
	RevisionID  uuid.UUID `json:"revision_id"`
}

func Register(registry *events.Registry) error {
	if err := events.RegisterJSON[FileCreated](
		registry,
		FileCreatedName,
		1,
		func(ev FileCreated) events.AggregateRef {
			return events.AggregateRef{Type: "file", ID: ev.FileID.String()}
		},
	); err != nil {
		return err
	}
	if err := events.RegisterJSON[FileRevisionCreated](
		registry,
		FileRevisionCreatedName,
		1,
		func(ev FileRevisionCreated) events.AggregateRef {
			return events.AggregateRef{Type: "file", ID: ev.FileID.String()}
		},
	); err != nil {
		return err
	}
	registrations := []func() error{
		func() error {
			return registerNodeEvent(registry, DirectoryCreatedName, func(event DirectoryCreated) uuid.UUID { return event.NodeID })
		},
		func() error {
			return registerNodeEvent(registry, NodeRenamedName, func(event NodeRenamed) uuid.UUID { return event.NodeID })
		},
		func() error {
			return registerNodeEvent(registry, NodeMovedName, func(event NodeMoved) uuid.UUID { return event.NodeID })
		},
		func() error {
			return registerNodeEvent(registry, NodeCopiedName, func(event NodeCopied) uuid.UUID { return event.NodeID })
		},
		func() error {
			return registerNodeEvent(registry, NodeTrashedName, func(event NodeTrashed) uuid.UUID { return event.NodeID })
		},
		func() error {
			return registerNodeEvent(registry, NodeRestoredName, func(event NodeRestored) uuid.UUID { return event.NodeID })
		},
		func() error {
			return registerNodeEvent(registry, NodePurgedName, func(event NodePurged) uuid.UUID { return event.NodeID })
		},
		func() error {
			return registerNodeEvent(registry, RevisionRestoredName, func(event RevisionRestored) uuid.UUID { return event.NodeID })
		},
		func() error {
			return registerNodeEvent(registry, PermissionChangedName, func(event PermissionChanged) uuid.UUID { return event.NodeID })
		},
		func() error {
			return registerTransferEvent(registry, TransferInitiatedName, func(event TransferInitiated) uuid.UUID { return event.SessionID })
		},
		func() error {
			return registerTransferEvent(registry, TransferUploadedName, func(event TransferUploaded) uuid.UUID { return event.SessionID })
		},
		func() error {
			return registerTransferEvent(registry, TransferCompletedName, func(event TransferCompleted) uuid.UUID { return event.SessionID })
		},
		func() error {
			return events.RegisterJSON[ArtifactCreated](registry, ArtifactCreatedName, 1, func(ev ArtifactCreated) events.AggregateRef {
				return events.AggregateRef{Type: "artifact", ID: ev.ArtifactID.String()}
			})
		},
	}
	for _, register := range registrations {
		if err := register(); err != nil {
			return err
		}
	}
	return nil
}

func registerNodeEvent[T events.Event](registry *events.Registry, name string, nodeID func(T) uuid.UUID) error {
	return events.RegisterJSON[T](registry, name, 1, func(event T) events.AggregateRef {
		return events.AggregateRef{Type: "file", ID: nodeID(event).String()}
	})
}

func registerTransferEvent[T events.Event](registry *events.Registry, name string, sessionID func(T) uuid.UUID) error {
	return events.RegisterJSON[T](registry, name, 1, func(event T) events.AggregateRef {
		return events.AggregateRef{Type: "transfer", ID: sessionID(event).String()}
	})
}
