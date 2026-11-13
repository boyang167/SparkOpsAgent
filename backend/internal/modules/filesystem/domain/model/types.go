package model

import (
	"errors"
	"time"

	"github.com/google/uuid"
)

var ErrInvalidNodeName = errors.New("invalid node name")

type Namespace struct {
	ID             uuid.UUID
	OrganizationID uuid.UUID
	WorkspaceID    uuid.UUID
	RootNodeID     *uuid.UUID
	Status         string
}

type NodeType string

const (
	NodeTypeFile      NodeType = "FILE"
	NodeTypeDirectory NodeType = "DIRECTORY"
)

type NodeState string

const (
	NodeStateActive        NodeState = "ACTIVE"
	NodeStateTrashed       NodeState = "TRASHED"
	NodeStateRetentionHold NodeState = "RETENTION_HOLD"
	NodeStatePurgePending  NodeState = "PURGE_PENDING"
	NodeStatePurged        NodeState = "PURGED"
)

type BlobState string

const (
	BlobStatePending     BlobState = "PENDING"
	BlobStateAvailable   BlobState = "AVAILABLE"
	BlobStateQuarantined BlobState = "QUARANTINED"
	BlobStateTombstoned  BlobState = "TOMBSTONED"
	BlobStateGCPending   BlobState = "GC_PENDING"
	BlobStateDeleted     BlobState = "DELETED"
)

type ReplicaState string

const (
	ReplicaStateCreating  ReplicaState = "CREATING"
	ReplicaStateVerifying ReplicaState = "VERIFYING"
	ReplicaStateAvailable ReplicaState = "AVAILABLE"
	ReplicaStateDraining  ReplicaState = "DRAINING"
	ReplicaStateFailed    ReplicaState = "FAILED"
	ReplicaStateDeleted   ReplicaState = "DELETED"
)

type Blob struct {
	ID                  uuid.UUID
	DedupDomain         uuid.UUID
	SHA256              string
	Size                int64
	EncryptionKeyRef    string
	RefCount            int64
	State               BlobState
	PreferredProviderID *uuid.UUID
}

type StorageReplica struct {
	ID         uuid.UUID
	BlobID     uuid.UUID
	ProviderID uuid.UUID
	ObjectKey  string
	Checksum   string
	State      ReplicaState
	VerifiedAt *time.Time
	DeleteAfter *time.Time
}

type GrantEffect string

const (
	GrantEffectAllow GrantEffect = "ALLOW"
	GrantEffectDeny  GrantEffect = "DENY"
)

type ResourcePermission string

const (
	ResourcePermissionRead   ResourcePermission = "READ"
	ResourcePermissionWrite  ResourcePermission = "WRITE"
	ResourcePermissionDelete ResourcePermission = "DELETE"
	ResourcePermissionShare  ResourcePermission = "SHARE"
	ResourcePermissionAdmin  ResourcePermission = "ADMIN"
)

type ResourceGrant struct {
	ID         uuid.UUID
	NodeID     uuid.UUID
	GranteeID  uuid.UUID
	Permission ResourcePermission
	Effect     GrantEffect
	ExpiresAt  *time.Time
}
