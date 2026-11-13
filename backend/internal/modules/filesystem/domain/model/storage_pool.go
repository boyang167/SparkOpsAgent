package model

import (
	"errors"
	"strings"

	"github.com/google/uuid"
)

var (
	ErrAmbiguousPrimaryProvider = errors.New("ambiguous primary provider priority")
	ErrInvalidStorageProvider   = errors.New("invalid storage provider")
	ErrInvalidStoragePool       = errors.New("invalid storage pool")
	ErrInvalidStorageBinding    = errors.New("invalid storage binding")
)

type ProviderType string

const (
	ProviderTypeLocal ProviderType = "LOCAL"
	ProviderTypeNFS   ProviderType = "NFS"
	ProviderTypeS3    ProviderType = "S3"
)

type ProviderMode string

const (
	ProviderModePrimary  ProviderMode = "PRIMARY"
	ProviderModeReplica  ProviderMode = "REPLICA"
	ProviderModeArchive  ProviderMode = "ARCHIVE"
	ProviderModeDraining ProviderMode = "DRAINING"
)

type StoragePoolStatus string

const (
	StoragePoolActive   StoragePoolStatus = "ACTIVE"
	StoragePoolDisabled StoragePoolStatus = "DISABLED"
)

type ScopeType string

const (
	ScopeOrganization ScopeType = "ORGANIZATION"
	ScopeWorkspace    ScopeType = "WORKSPACE"
	ScopeProject      ScopeType = "PROJECT"
	ScopeUser         ScopeType = "USER"
)

type MountAccessMode string

const (
	MountAccessReadOnly  MountAccessMode = "READ_ONLY"
	MountAccessReadWrite MountAccessMode = "READ_WRITE"
)

type StorageProvider struct {
	ID             uuid.UUID
	OrganizationID uuid.UUID
	Name           string
	ProviderType   ProviderType
	Endpoint       string
	SecretRef      string
	Capabilities   map[string]any
	Health         map[string]any
	ConfigVersion  int64
	Status         string
}

type StorageProviderInput struct {
	ID             uuid.UUID
	OrganizationID uuid.UUID
	Name           string
	ProviderType   ProviderType
	Endpoint       string
	SecretRef      string
	Capabilities   map[string]any
	Health         map[string]any
	ConfigVersion  int64
	Status         string
}

func NewStorageProvider(input StorageProviderInput) (*StorageProvider, error) {
	if input.ID == uuid.Nil || input.OrganizationID == uuid.Nil {
		return nil, ErrInvalidStorageProvider
	}
	name := strings.TrimSpace(input.Name)
	if name == "" {
		return nil, ErrInvalidStorageProvider
	}
	secretRef := strings.TrimSpace(input.SecretRef)
	if secretRef == "" {
		return nil, ErrInvalidStorageProvider
	}
	switch input.ProviderType {
	case ProviderTypeLocal, ProviderTypeNFS, ProviderTypeS3:
	default:
		return nil, ErrInvalidStorageProvider
	}
	status := strings.TrimSpace(input.Status)
	if status == "" {
		status = "ACTIVE"
	}
	configVersion := input.ConfigVersion
	if configVersion <= 0 {
		configVersion = 1
	}
	capabilities := input.Capabilities
	if capabilities == nil {
		capabilities = map[string]any{}
	}
	health := input.Health
	if health == nil {
		health = map[string]any{}
	}
	return &StorageProvider{
		ID:             input.ID,
		OrganizationID: input.OrganizationID,
		Name:           name,
		ProviderType:   input.ProviderType,
		Endpoint:       strings.TrimSpace(input.Endpoint),
		SecretRef:      secretRef,
		Capabilities:   capabilities,
		Health:         health,
		ConfigVersion:  configVersion,
		Status:         status,
	}, nil
}

type StoragePool struct {
	id                     uuid.UUID
	name                   string
	placementPolicyVersion int64
	status                 StoragePoolStatus
	members                []storagePoolMember
}

type storagePoolMember struct {
	providerID uuid.UUID
	mode       ProviderMode
	priority   int
}

type StoragePoolSnapshot struct {
	ID                     uuid.UUID
	Name                   string
	PlacementPolicyVersion int64
	Status                 StoragePoolStatus
	Members                []StoragePoolMember
}

type StoragePoolMember struct {
	ProviderID uuid.UUID
	Mode       ProviderMode
	Priority   int
}

func NewStoragePool(id uuid.UUID, name string) *StoragePool {
	return &StoragePool{
		id:                     id,
		name:                   strings.TrimSpace(name),
		placementPolicyVersion: 0,
		status:                 StoragePoolActive,
	}
}

func (p *StoragePool) ID() uuid.UUID {
	return p.id
}

func (p *StoragePool) Name() string {
	return p.name
}

func (p *StoragePool) Status() StoragePoolStatus {
	return p.status
}

func (p *StoragePool) PlacementPolicyVersion() int64 {
	return p.placementPolicyVersion
}

func (p *StoragePool) Members() []StoragePoolMember {
	out := make([]StoragePoolMember, len(p.members))
	for i, member := range p.members {
		out[i] = StoragePoolMember{
			ProviderID: member.providerID,
			Mode:       member.mode,
			Priority:   member.priority,
		}
	}
	return out
}

func RestoreStoragePool(snapshot StoragePoolSnapshot) *StoragePool {
	pool := &StoragePool{
		id:                     snapshot.ID,
		name:                   snapshot.Name,
		placementPolicyVersion: snapshot.PlacementPolicyVersion,
		status:                 snapshot.Status,
	}
	for _, member := range snapshot.Members {
		pool.members = append(pool.members, storagePoolMember{
			providerID: member.ProviderID,
			mode:       member.Mode,
			priority:   member.Priority,
		})
	}
	return pool
}

func (p *StoragePool) Snapshot() StoragePoolSnapshot {
	snap := StoragePoolSnapshot{
		ID:                     p.id,
		Name:                   p.name,
		PlacementPolicyVersion: p.placementPolicyVersion,
		Status:                 p.status,
		Members:                p.Members(),
	}
	return snap
}

func (p *StoragePool) AddProvider(providerID uuid.UUID, mode ProviderMode, priority int) error {
	if providerID == uuid.Nil {
		return ErrInvalidStoragePool
	}
	switch mode {
	case ProviderModePrimary, ProviderModeReplica, ProviderModeArchive, ProviderModeDraining:
	default:
		return ErrInvalidStoragePool
	}
	if mode == ProviderModePrimary {
		for _, member := range p.members {
			if member.mode == ProviderModePrimary && member.priority == priority {
				return ErrAmbiguousPrimaryProvider
			}
		}
	}
	p.members = append(p.members, storagePoolMember{
		providerID: providerID,
		mode:       mode,
		priority:   priority,
	})
	return nil
}

type StorageBinding struct {
	ID             uuid.UUID
	ScopeType      ScopeType
	ScopeID        uuid.UUID
	PoolID         uuid.UUID
	Priority       int
	OrganizationID uuid.UUID
}

type StorageBindingInput struct {
	ID             uuid.UUID
	ScopeType      ScopeType
	ScopeID        uuid.UUID
	PoolID         uuid.UUID
	Priority       int
	OrganizationID uuid.UUID
}

func NewStorageBinding(input StorageBindingInput) (*StorageBinding, error) {
	if input.ID == uuid.Nil || input.ScopeID == uuid.Nil || input.PoolID == uuid.Nil || input.OrganizationID == uuid.Nil {
		return nil, ErrInvalidStorageBinding
	}
	switch input.ScopeType {
	case ScopeOrganization, ScopeWorkspace, ScopeProject, ScopeUser:
	default:
		return nil, ErrInvalidStorageBinding
	}
	return &StorageBinding{
		ID:             input.ID,
		ScopeType:      input.ScopeType,
		ScopeID:        input.ScopeID,
		PoolID:         input.PoolID,
		Priority:       input.Priority,
		OrganizationID: input.OrganizationID,
	}, nil
}

type PlacementPolicyStatus string

const (
	PlacementPolicyActive   PlacementPolicyStatus = "ACTIVE"
	PlacementPolicyDisabled PlacementPolicyStatus = "DISABLED"
)

type PlacementPolicy struct {
	ID             uuid.UUID
	OrganizationID uuid.UUID
	Name           string
	Version        int64
	Rules          map[string]any
	Status         PlacementPolicyStatus
}
