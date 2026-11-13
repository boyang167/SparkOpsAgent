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
	VerifiedAt *time.Time         `gorm:"column:verified_at"`
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

type quotaReservationRow struct {
	ID             uuid.UUID  `gorm:"column:id;primaryKey;type:uuid"`
	QuotaAccountID uuid.UUID  `gorm:"column:quota_account_id;not null;type:uuid;index"`
	WorkspaceID    uuid.UUID  `gorm:"column:workspace_id;not null;type:uuid;index"`
	SessionID      *uuid.UUID `gorm:"column:session_id;type:uuid;uniqueIndex"`
	Bytes          int64      `gorm:"not null"`
	Files          int64      `gorm:"not null"`
	State          string     `gorm:"not null;type:varchar(32)"`
	ExpiresAt      time.Time  `gorm:"column:expires_at;not null"`
	CreatedAt      time.Time  `gorm:"column:created_at;not null"`
}

func (quotaReservationRow) TableName() string { return "filesystem.quota_reservations" }

type resourceGrantRow struct {
	ID         uuid.UUID                `gorm:"column:id;primaryKey;type:uuid"`
	NodeID     uuid.UUID                `gorm:"column:node_id;not null;type:uuid;index"`
	GranteeID  uuid.UUID                `gorm:"column:grantee_id;not null;type:uuid;index"`
	Permission model.ResourcePermission `gorm:"not null;type:varchar(32)"`
	Effect     model.GrantEffect        `gorm:"not null;type:varchar(8)"`
	ExpiresAt  *time.Time               `gorm:"column:expires_at"`
	CreatedAt  time.Time                `gorm:"column:created_at;not null"`
}

func (resourceGrantRow) TableName() string { return "filesystem.resource_grants" }

type fileLinkRow struct {
	ID         uuid.UUID  `gorm:"column:id;primaryKey;type:uuid"`
	NodeID     uuid.UUID  `gorm:"column:node_id;not null;type:uuid;index"`
	RevisionID *uuid.UUID `gorm:"column:revision_id;type:uuid"`
	LinkType   string     `gorm:"column:link_type;not null;type:varchar(64)"`
	LinkID     uuid.UUID  `gorm:"column:link_id;not null;type:uuid"`
	CreatedAt  time.Time  `gorm:"column:created_at;not null"`
}

func (fileLinkRow) TableName() string { return "filesystem.file_links" }

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

type transferSessionRow struct {
	ID                 uuid.UUID                    `gorm:"column:id;primaryKey;type:uuid"`
	WorkspaceID        uuid.UUID                    `gorm:"column:workspace_id;not null;type:uuid;uniqueIndex:ux_transfer_idempotency"`
	NamespaceID        uuid.UUID                    `gorm:"column:namespace_id;not null;type:uuid"`
	TargetParentID     *uuid.UUID                   `gorm:"column:target_parent_id;type:uuid"`
	TargetName         string                       `gorm:"column:target_name;not null"`
	Protocol           model.TransferProtocol       `gorm:"column:protocol;not null;type:varchar(24)"`
	ExpectedSize       int64                        `gorm:"column:expected_size;not null"`
	ExpectedSHA256     *string                      `gorm:"column:expected_sha256;size:64"`
	ActualSize         *int64                       `gorm:"column:actual_size"`
	ActualSHA256       *string                      `gorm:"column:actual_sha256;size:64"`
	ProviderID         *uuid.UUID                   `gorm:"column:provider_id;type:uuid"`
	TemporaryObjectKey string                       `gorm:"column:temporary_object_key;not null"`
	Status             model.TransferStatus         `gorm:"column:status;not null;type:varchar(24)"`
	Classification     model.TransferClassification `gorm:"column:classification;not null;type:varchar(16)"`
	ScanClean          *bool                        `gorm:"column:scan_clean"`
	ScanThreatName     string                       `gorm:"column:scan_threat_name"`
	ScanEngine         string                       `gorm:"column:scan_engine"`
	ScanSignature      string                       `gorm:"column:scan_signature"`
	ScanCompletedAt    *time.Time                   `gorm:"column:scan_completed_at"`
	ErrorCode          string                       `gorm:"column:error_code"`
	IdempotencyKey     string                       `gorm:"column:idempotency_key;not null;uniqueIndex:ux_transfer_idempotency"`
	ExpiresAt          time.Time                    `gorm:"column:expires_at;not null"`
	Version            int64                        `gorm:"column:version;not null"`
	CreatedBy          uuid.UUID                    `gorm:"column:created_by;not null;type:uuid"`
	CreatedAt          time.Time                    `gorm:"column:created_at;not null"`
	UpdatedAt          time.Time                    `gorm:"column:updated_at;not null"`
}

func (transferSessionRow) TableName() string { return "filesystem.transfer_sessions" }

type transferPartRow struct {
	ID         uuid.UUID `gorm:"column:id;primaryKey;type:uuid"`
	SessionID  uuid.UUID `gorm:"column:session_id;not null;type:uuid;uniqueIndex:ux_transfer_part_number"`
	PartNumber int       `gorm:"column:part_number;not null;uniqueIndex:ux_transfer_part_number"`
	Offset     int64     `gorm:"column:offset_bytes;not null"`
	Size       int64     `gorm:"column:size_bytes;not null"`
	Checksum   string    `gorm:"column:checksum;size:64"`
	ETag       string    `gorm:"column:etag;not null"`
	CreatedAt  time.Time `gorm:"column:created_at;not null"`
	UpdatedAt  time.Time `gorm:"column:updated_at;not null"`
}

func (transferPartRow) TableName() string { return "filesystem.transfer_parts" }
