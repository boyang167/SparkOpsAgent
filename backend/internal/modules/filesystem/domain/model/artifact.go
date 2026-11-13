package model

import (
	"time"

	"github.com/google/uuid"
)

const FileLinkTypeArtifact = "ARTIFACT"

// Artifact is a business-facing link to a file node (not a duplicate blob).
type Artifact struct {
	ID                uuid.UUID
	WorkspaceID       uuid.UUID
	NamespaceID       uuid.UUID
	ArtifactType      string
	ProducerType      string
	ProducerID        uuid.UUID
	RunID             uuid.UUID
	InputRefs         []uuid.UUID
	FileID            uuid.UUID
	RevisionID        *uuid.UUID
	RetentionPolicyID *uuid.UUID
	CreatedAt         time.Time
}

// FileLink records an external reference to a file node or revision.
type FileLink struct {
	ID         uuid.UUID
	NodeID     uuid.UUID
	RevisionID *uuid.UUID
	LinkType   string
	LinkID     uuid.UUID
	CreatedAt  time.Time
}
