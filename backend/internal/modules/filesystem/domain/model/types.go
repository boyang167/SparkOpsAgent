package model

import (
	"errors"

	"github.com/google/uuid"
)

var ErrInvalidNodeName = errors.New("invalid node name")

type NodeType string

const (
	NodeTypeFile      NodeType = "FILE"
	NodeTypeDirectory NodeType = "DIRECTORY"
)

type NodeState string

const (
	NodeStateActive NodeState = "ACTIVE"
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
	ID               uuid.UUID
	DedupDomain      uuid.UUID
	SHA256           string
	Size             int64
	EncryptionKeyRef string
	RefCount         int64
	State            BlobState
}

type StorageReplica struct {
	ID         uuid.UUID
	BlobID     uuid.UUID
	ProviderID uuid.UUID
	ObjectKey  string
	Checksum   string
	State      ReplicaState
}
