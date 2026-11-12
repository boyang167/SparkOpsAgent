package persistence

import (
	"time"

	"github.com/google/uuid"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/domain/model"
)

type namespaceRow struct {
	ID             uuid.UUID  `gorm:"column:id;primaryKey;type:uuid"`
	OrganizationID uuid.UUID  `gorm:"column:organization_id;not null;type:uuid"`
	WorkspaceID    uuid.UUID  `gorm:"column:workspace_id;not null;uniqueIndex;type:uuid"`
	RootNodeID     *uuid.UUID `gorm:"column:root_node_id;type:uuid"`
	Status         string     `gorm:"not null;type:varchar(32)"`
	CreatedAt      time.Time  `gorm:"not null"`
	UpdatedAt      time.Time  `gorm:"not null"`
}

func (namespaceRow) TableName() string { return "filesystem.namespaces" }

type nodeRow struct {
	ID                uuid.UUID       `gorm:"column:id;primaryKey;type:uuid"`
	NamespaceID       uuid.UUID       `gorm:"column:namespace_id;not null;type:uuid;index"`
	ParentID          *uuid.UUID      `gorm:"column:parent_id;type:uuid"`
	Type              model.NodeType  `gorm:"column:node_type;not null;type:varchar(16)"`
	Name              string          `gorm:"not null"`
	NormalizedName    string          `gorm:"column:normalized_name;not null;index"`
	CurrentRevisionID *uuid.UUID      `gorm:"column:current_revision_id;type:uuid"`
	State             model.NodeState `gorm:"not null;type:varchar(32)"`
	OwnerID           uuid.UUID       `gorm:"column:owner_id;not null;type:uuid"`
	CreatedAt         time.Time       `gorm:"not null"`
	UpdatedAt         time.Time       `gorm:"not null"`
}

func (nodeRow) TableName() string { return "filesystem.nodes" }

type blobRow struct {
	ID               uuid.UUID       `gorm:"column:id;primaryKey;type:uuid"`
	DedupDomain      uuid.UUID       `gorm:"column:dedup_domain;not null;type:uuid"`
	SHA256           string          `gorm:"column:sha256;not null;size:64"`
	Size             int64           `gorm:"not null"`
	EncryptionKeyRef string          `gorm:"column:encryption_key_ref"`
	RefCount         int64           `gorm:"column:ref_count;not null"`
	State            model.BlobState `gorm:"not null;type:varchar(32)"`
	CreatedAt        time.Time       `gorm:"not null"`
}

func (blobRow) TableName() string { return "filesystem.blobs" }

type revisionRow struct {
	ID         uuid.UUID `gorm:"column:id;primaryKey;type:uuid"`
	NodeID     uuid.UUID `gorm:"column:node_id;not null;type:uuid"`
	RevisionNo int       `gorm:"column:revision_no;not null"`
	BlobID     uuid.UUID `gorm:"column:blob_id;not null;type:uuid"`
	Size       int64     `gorm:"not null"`
	MIMEType   string    `gorm:"column:mime_type;not null;size:255"`
	CreatedBy  uuid.UUID `gorm:"column:created_by;not null;type:uuid"`
	CreatedAt  time.Time `gorm:"not null"`
}

func (revisionRow) TableName() string { return "filesystem.file_revisions" }

type replicaRow struct {
	ID         uuid.UUID          `gorm:"column:id;primaryKey;type:uuid"`
	BlobID     uuid.UUID          `gorm:"column:blob_id;not null;type:uuid"`
	ProviderID uuid.UUID          `gorm:"column:provider_id;not null;type:uuid"`
	ObjectKey  string             `gorm:"column:object_key;not null"`
	Checksum   string             `gorm:"column:checksum"`
	State      model.ReplicaState `gorm:"not null;type:varchar(32)"`
	CreatedAt  time.Time          `gorm:"not null"`
}

func (replicaRow) TableName() string { return "filesystem.storage_replicas" }

type quotaAccountRow struct {
	ID             uuid.UUID `gorm:"column:id;primaryKey;type:uuid"`
	OrganizationID uuid.UUID `gorm:"column:organization_id;not null;type:uuid"`
	WorkspaceID    uuid.UUID `gorm:"column:workspace_id;not null;uniqueIndex;type:uuid"`
	UsedBytes      int64     `gorm:"column:used_bytes;not null"`
	ReservedBytes  int64     `gorm:"column:reserved_bytes;not null"`
	UsedFiles      int64     `gorm:"column:used_files;not null"`
	ReservedFiles  int64     `gorm:"column:reserved_files;not null"`
	MaxBytes       *int64    `gorm:"column:max_bytes"`
	MaxFiles       *int64    `gorm:"column:max_files"`
	Version        int64     `gorm:"column:version;not null"`
	UpdatedAt      time.Time `gorm:"column:updated_at;not null"`
}

func (quotaAccountRow) TableName() string { return "filesystem.quota_accounts" }

type commandDedupRow struct {
	CommandID   uuid.UUID  `gorm:"column:command_id;primaryKey;type:uuid"`
	WorkspaceID uuid.UUID  `gorm:"column:workspace_id;not null;type:uuid"`
	Operation   string     `gorm:"column:operation;not null"`
	RequestHash string     `gorm:"column:request_hash;not null;size:64"`
	NodeID      *uuid.UUID `gorm:"column:node_id;type:uuid"`
	RevisionID  *uuid.UUID `gorm:"column:revision_id;type:uuid"`
	State       string     `gorm:"column:state;not null;size:16"`
	CreatedAt   time.Time  `gorm:"column:created_at;not null"`
	UpdatedAt   time.Time  `gorm:"column:updated_at;not null"`
}

func (commandDedupRow) TableName() string { return "filesystem.command_dedup" }
