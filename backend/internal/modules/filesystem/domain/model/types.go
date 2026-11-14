package model

import (
	"errors"
	"time"

	"github.com/google/uuid"
)

var ErrInvalidNodeName = errors.New("invalid node name")
var ErrInvalidNamespace = errors.New("invalid namespace")

type NamespaceType string

const (
	NamespaceTypeOrganization NamespaceType = "ORGANIZATION"
	NamespaceTypeWorkspace    NamespaceType = "WORKSPACE"
	NamespaceTypeUser         NamespaceType = "USER"
)

type Namespace struct {
	ID             uuid.UUID
	NamespaceType  NamespaceType
	OrganizationID uuid.UUID
	WorkspaceID    uuid.UUID
	OwnerUserID    uuid.UUID
	RootNodeID     *uuid.UUID
	Status         string
	PolicyVersion  int64
}

func (n Namespace) Validate() error {
	switch n.NamespaceType {
	case NamespaceTypeOrganization:
		if n.OrganizationID == uuid.Nil || n.WorkspaceID != uuid.Nil || n.OwnerUserID != uuid.Nil {
			return ErrInvalidNamespace
		}
	case NamespaceTypeWorkspace:
		if n.OrganizationID == uuid.Nil || n.WorkspaceID == uuid.Nil || n.OwnerUserID != uuid.Nil {
			return ErrInvalidNamespace
		}
	case NamespaceTypeUser:
		if n.OrganizationID == uuid.Nil || n.WorkspaceID != uuid.Nil || n.OwnerUserID == uuid.Nil {
			return ErrInvalidNamespace
		}
	default:
		return ErrInvalidNamespace
	}
	return nil
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
	TombstonedAt        *time.Time
	LegalHold           bool
}

type StorageReplica struct {
	ID          uuid.UUID
	BlobID      uuid.UUID
	ProviderID  uuid.UUID
	ObjectKey   string
	Checksum    string
	State       ReplicaState
	VerifiedAt  *time.Time
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

type PrincipalType string

const (
	PrincipalUser           PrincipalType = "USER"
	PrincipalWorkspace      PrincipalType = "WORKSPACE"
	PrincipalOrganization   PrincipalType = "ORGANIZATION"
	PrincipalRole           PrincipalType = "ROLE"
	PrincipalServiceAccount PrincipalType = "SERVICE_ACCOUNT"
	PrincipalAgent          PrincipalType = "AGENT"
)

type ResourceGrant struct {
	ID                uuid.UUID
	NodeID            uuid.UUID
	OrganizationID    uuid.UUID
	GranteeID         uuid.UUID
	PrincipalType     PrincipalType
	PrincipalID       uuid.UUID
	Permission        ResourcePermission
	Effect            GrantEffect
	InheritToChildren bool
	ExpiresAt         *time.Time
	RevokedAt         *time.Time
	PolicyVersion     int64
}
