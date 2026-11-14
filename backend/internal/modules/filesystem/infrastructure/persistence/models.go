package persistence

import (
	"time"

	"github.com/google/uuid"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/domain/model"
)

type namespaceRow struct {
	ID             uuid.UUID           `gorm:"column:id;primaryKey;type:uuid"`
	NamespaceType  model.NamespaceType `gorm:"column:namespace_type;not null;default:'WORKSPACE';type:varchar(16)"`
	OrganizationID uuid.UUID           `gorm:"column:organization_id;not null;type:uuid"`
	WorkspaceID    *uuid.UUID          `gorm:"column:workspace_id;type:uuid"`
	OwnerUserID    *uuid.UUID          `gorm:"column:owner_user_id;type:uuid"`
	RootNodeID     *uuid.UUID          `gorm:"column:root_node_id;type:uuid"`
	Status         string              `gorm:"not null;type:varchar(32)"`
	PolicyVersion  int64               `gorm:"column:policy_version;not null;default:1"`
	CreatedAt      time.Time           `gorm:"not null"`
	UpdatedAt      time.Time           `gorm:"not null"`
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
	ID                  uuid.UUID       `gorm:"column:id;primaryKey;type:uuid"`
	DedupDomain         uuid.UUID       `gorm:"column:dedup_domain;not null;type:uuid"`
	SHA256              string          `gorm:"column:sha256;not null;size:64"`
	Size                int64           `gorm:"not null"`
	EncryptionKeyRef    string          `gorm:"column:encryption_key_ref"`
	RefCount            int64           `gorm:"column:ref_count;not null"`
	State               model.BlobState `gorm:"not null;type:varchar(32)"`
	PreferredProviderID *uuid.UUID      `gorm:"column:preferred_provider_id;type:uuid"`
	TombstonedAt        *time.Time      `gorm:"column:tombstoned_at"`
	LegalHold           bool            `gorm:"column:legal_hold;not null;default:false"`
	CreatedAt           time.Time       `gorm:"not null"`
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
	ID          uuid.UUID          `gorm:"column:id;primaryKey;type:uuid"`
	BlobID      uuid.UUID          `gorm:"column:blob_id;not null;type:uuid"`
	ProviderID  uuid.UUID          `gorm:"column:provider_id;not null;type:uuid"`
	ObjectKey   string             `gorm:"column:object_key;not null"`
	Checksum    string             `gorm:"column:checksum"`
	State       model.ReplicaState `gorm:"not null;type:varchar(32)"`
	VerifiedAt  *time.Time         `gorm:"column:verified_at"`
	DeleteAfter *time.Time         `gorm:"column:delete_after"`
	CreatedAt   time.Time          `gorm:"not null"`
}

type replicaMigrationRow struct {
	BlobID           uuid.UUID `gorm:"column:blob_id;primaryKey;type:uuid"`
	TargetProviderID uuid.UUID `gorm:"column:target_provider_id;primaryKey;type:uuid"`
	TargetReplicaID  uuid.UUID `gorm:"column:target_replica_id;not null;type:uuid"`
	SourceProviderID uuid.UUID `gorm:"column:source_provider_id;not null;type:uuid"`
	BytesCopied      int64     `gorm:"column:bytes_copied;not null"`
	TargetObjectKey  string    `gorm:"column:target_object_key;not null"`
	PolicyVersion    int64     `gorm:"column:policy_version;not null"`
	UpdatedAt        time.Time `gorm:"column:updated_at;not null"`
}

func (replicaMigrationRow) TableName() string { return "filesystem.replica_migrations" }

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
	ID                uuid.UUID                `gorm:"column:id;primaryKey;type:uuid"`
	NodeID            uuid.UUID                `gorm:"column:node_id;not null;type:uuid;index"`
	OrganizationID    uuid.UUID                `gorm:"column:organization_id;type:uuid;index"`
	GranteeID         uuid.UUID                `gorm:"column:grantee_id;not null;type:uuid;index"`
	PrincipalType     model.PrincipalType      `gorm:"column:principal_type;type:varchar(32);default:USER"`
	PrincipalID       uuid.UUID                `gorm:"column:principal_id;type:uuid;index"`
	Permission        model.ResourcePermission `gorm:"not null;type:varchar(32)"`
	Effect            model.GrantEffect        `gorm:"not null;type:varchar(8)"`
	InheritToChildren bool                     `gorm:"column:inherit_to_children;not null;default:true"`
	ExpiresAt         *time.Time               `gorm:"column:expires_at"`
	RevokedAt         *time.Time               `gorm:"column:revoked_at"`
	PolicyVersion     int64                    `gorm:"column:policy_version;not null;default:1"`
	CreatedAt         time.Time                `gorm:"column:created_at;not null"`
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
	WorkspaceID        uuid.UUID                    `gorm:"column:workspace_id;not null;type:uuid;uniqueIndex:ux_transfer_idempotency;index:ix_transfer_sessions_workspace_status,priority:1"`
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
	Status             model.TransferStatus         `gorm:"column:status;not null;type:varchar(24);index:ix_transfer_sessions_workspace_status,priority:2"`
	Classification     model.TransferClassification `gorm:"column:classification;not null;type:varchar(16)"`
	ScanClean          *bool                        `gorm:"column:scan_clean"`
	ScanThreatName     string                       `gorm:"column:scan_threat_name"`
	ScanEngine         string                       `gorm:"column:scan_engine"`
	ScanSignature      string                       `gorm:"column:scan_signature"`
	ScanCompletedAt    *time.Time                   `gorm:"column:scan_completed_at"`
	ErrorCode          string                       `gorm:"column:error_code"`
	IdempotencyKey     string                       `gorm:"column:idempotency_key;not null;uniqueIndex:ux_transfer_idempotency"`
	RequestHash        string                       `gorm:"column:request_hash;not null;size:64"`
	ExpiresAt          time.Time                    `gorm:"column:expires_at;not null;index:ix_transfer_sessions_expires_at"`
	Version            int64                        `gorm:"column:version;not null"`
	CreatedBy          uuid.UUID                    `gorm:"column:created_by;not null;type:uuid"`
	CreatedAt          time.Time                    `gorm:"column:created_at;not null"`
	UpdatedAt          time.Time                    `gorm:"column:updated_at;not null"`
}

func (transferSessionRow) TableName() string { return "filesystem.transfer_sessions" }

type transferPartRow struct {
	ID         uuid.UUID `gorm:"column:id;primaryKey;type:uuid"`
	SessionID  uuid.UUID `gorm:"column:session_id;not null;type:uuid;uniqueIndex:ux_transfer_part_number;index:ix_transfer_parts_session_offset,priority:1"`
	PartNumber int       `gorm:"column:part_number;not null;uniqueIndex:ux_transfer_part_number"`
	Offset     int64     `gorm:"column:offset_bytes;not null;index:ix_transfer_parts_session_offset,priority:2"`
	Size       int64     `gorm:"column:size_bytes;not null"`
	Checksum   *string   `gorm:"column:checksum;size:64"`
	ETag       string    `gorm:"column:etag;not null"`
	CreatedAt  time.Time `gorm:"column:created_at;not null"`
	UpdatedAt  time.Time `gorm:"column:updated_at;not null"`
}

func (transferPartRow) TableName() string { return "filesystem.transfer_parts" }

type storageProviderRow struct {
	ID             uuid.UUID          `gorm:"column:id;primaryKey;type:uuid"`
	OrganizationID uuid.UUID          `gorm:"column:organization_id;not null;type:uuid;uniqueIndex:ux_storage_provider_org_name"`
	Name           string             `gorm:"column:name;not null;uniqueIndex:ux_storage_provider_org_name"`
	ProviderType   model.ProviderType `gorm:"column:provider_type;not null;type:varchar(16)"`
	Endpoint       string             `gorm:"column:endpoint;not null"`
	SecretRef      string             `gorm:"column:secret_ref;not null"`
	Capabilities   jsonObject         `gorm:"column:capabilities;not null;type:text"`
	Health         jsonObject         `gorm:"column:health;not null;type:text"`
	ConfigVersion  int64              `gorm:"column:config_version;not null"`
	Status         string             `gorm:"column:status;not null;type:varchar(32)"`
	CreatedAt      time.Time          `gorm:"column:created_at;not null"`
	UpdatedAt      time.Time          `gorm:"column:updated_at;not null"`
}

func (storageProviderRow) TableName() string { return "filesystem.storage_providers" }

type storagePoolRow struct {
	ID                     uuid.UUID               `gorm:"column:id;primaryKey;type:uuid"`
	OrganizationID         uuid.UUID               `gorm:"column:organization_id;not null;type:uuid;uniqueIndex:ux_storage_pool_org_name"`
	Name                   string                  `gorm:"column:name;not null;uniqueIndex:ux_storage_pool_org_name"`
	Status                 model.StoragePoolStatus `gorm:"column:status;not null;type:varchar(32)"`
	PlacementPolicyVersion int64                   `gorm:"column:placement_policy_version;not null"`
	CreatedAt              time.Time               `gorm:"column:created_at;not null"`
	UpdatedAt              time.Time               `gorm:"column:updated_at;not null"`
}

func (storagePoolRow) TableName() string { return "filesystem.storage_pools" }

type storagePoolMemberRow struct {
	ID         uuid.UUID          `gorm:"column:id;primaryKey;type:uuid"`
	PoolID     uuid.UUID          `gorm:"column:pool_id;not null;type:uuid;uniqueIndex:ux_storage_pool_member"`
	ProviderID uuid.UUID          `gorm:"column:provider_id;not null;type:uuid;uniqueIndex:ux_storage_pool_member"`
	Priority   int                `gorm:"column:priority;not null"`
	Mode       model.ProviderMode `gorm:"column:mode;not null;type:varchar(16)"`
	CreatedAt  time.Time          `gorm:"column:created_at;not null"`
}

func (storagePoolMemberRow) TableName() string { return "filesystem.storage_pool_members" }

type storageBindingRow struct {
	ID             uuid.UUID       `gorm:"column:id;primaryKey;type:uuid"`
	OrganizationID uuid.UUID       `gorm:"column:organization_id;not null;type:uuid;uniqueIndex:ux_storage_binding_scope"`
	ScopeType      model.ScopeType `gorm:"column:scope_type;not null;type:varchar(16);uniqueIndex:ux_storage_binding_scope"`
	ScopeID        uuid.UUID       `gorm:"column:scope_id;not null;type:uuid;uniqueIndex:ux_storage_binding_scope"`
	PoolID         uuid.UUID       `gorm:"column:pool_id;not null;type:uuid"`
	Priority       int             `gorm:"column:priority;not null;uniqueIndex:ux_storage_binding_scope"`
	CreatedAt      time.Time       `gorm:"column:created_at;not null"`
}

func (storageBindingRow) TableName() string { return "filesystem.storage_bindings" }

type lifecyclePolicyRow struct {
	ID             uuid.UUID                   `gorm:"column:id;primaryKey;type:uuid"`
	OrganizationID uuid.UUID                   `gorm:"column:organization_id;not null;type:uuid;uniqueIndex:ux_lifecycle_policy_org_name_version"`
	Name           string                      `gorm:"column:name;not null;uniqueIndex:ux_lifecycle_policy_org_name_version"`
	Version        int64                       `gorm:"column:version;not null;uniqueIndex:ux_lifecycle_policy_org_name_version"`
	Rules          jsonObject                  `gorm:"column:rules;not null;type:text"`
	Status         model.LifecyclePolicyStatus `gorm:"column:status;not null;type:varchar(32)"`
	CreatedAt      time.Time                   `gorm:"column:created_at;not null"`
	UpdatedAt      time.Time                   `gorm:"column:updated_at;not null"`
}

func (lifecyclePolicyRow) TableName() string { return "filesystem.lifecycle_policies" }

type capabilityRow struct {
	ID             uuid.UUID  `gorm:"column:id;primaryKey;type:uuid"`
	TokenHash      string     `gorm:"column:token_hash;not null;uniqueIndex"`
	OrganizationID uuid.UUID  `gorm:"column:organization_id;not null;type:uuid"`
	WorkspaceID    uuid.UUID  `gorm:"column:workspace_id;not null;type:uuid"`
	ActorID        uuid.UUID  `gorm:"column:actor_id;not null;type:uuid"`
	AgentID        *uuid.UUID `gorm:"column:agent_id;type:uuid"`
	RunID          uuid.UUID  `gorm:"column:run_id;not null;type:uuid"`
	ResourceIDs    string     `gorm:"column:resource_ids;not null;type:text"`
	Actions        string     `gorm:"column:actions;not null;type:text"`
	Audience       string     `gorm:"column:audience;not null"`
	ExpiresAt      time.Time  `gorm:"column:expires_at;not null"`
	RevokedAt      *time.Time `gorm:"column:revoked_at"`
	CreatedAt      time.Time  `gorm:"column:created_at;not null"`
}

func (capabilityRow) TableName() string { return "filesystem.capabilities" }

type exportTaskRow struct {
	ID                   uuid.UUID  `gorm:"column:id;primaryKey;type:uuid"`
	SourceOrganizationID uuid.UUID  `gorm:"column:source_organization_id;not null;type:uuid"`
	TargetOrganizationID uuid.UUID  `gorm:"column:target_organization_id;not null;type:uuid"`
	SourceWorkspaceID    uuid.UUID  `gorm:"column:source_workspace_id;not null;type:uuid"`
	TargetWorkspaceID    uuid.UUID  `gorm:"column:target_workspace_id;not null;type:uuid"`
	SourceNodeID         uuid.UUID  `gorm:"column:source_node_id;not null;type:uuid"`
	TargetNodeID         *uuid.UUID `gorm:"column:target_node_id;type:uuid"`
	Status               string     `gorm:"column:status;not null"`
	ActorID              uuid.UUID  `gorm:"column:actor_id;not null;type:uuid"`
	CreatedAt            time.Time  `gorm:"column:created_at;not null"`
	UpdatedAt            time.Time  `gorm:"column:updated_at;not null"`
}

func (exportTaskRow) TableName() string { return "filesystem.export_tasks" }

type artifactRow struct {
	ID                uuid.UUID  `gorm:"column:id;primaryKey;type:uuid"`
	WorkspaceID       uuid.UUID  `gorm:"column:workspace_id;not null;type:uuid;index"`
	NamespaceID       uuid.UUID  `gorm:"column:namespace_id;not null;type:uuid"`
	ArtifactType      string     `gorm:"column:artifact_type;not null"`
	ProducerType      string     `gorm:"column:producer_type;not null;index"`
	ProducerID        uuid.UUID  `gorm:"column:producer_id;not null;type:uuid"`
	RunID             uuid.UUID  `gorm:"column:run_id;not null;type:uuid;index"`
	InputRefs         string     `gorm:"column:input_refs;not null;default:'[]'"`
	FileID            uuid.UUID  `gorm:"column:file_id;not null;type:uuid;index"`
	RevisionID        *uuid.UUID `gorm:"column:revision_id;type:uuid"`
	RetentionPolicyID *uuid.UUID `gorm:"column:retention_policy_id;type:uuid"`
	CreatedAt         time.Time  `gorm:"column:created_at;not null"`
}

func (artifactRow) TableName() string { return "filesystem.artifacts" }
