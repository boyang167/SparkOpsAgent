package model

import (
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
)

var (
	ErrInvalidLifecyclePolicy = errors.New("invalid lifecycle policy")
	ErrInvalidLifecycleJob    = errors.New("invalid lifecycle job")
)

type LifecyclePolicyStatus string

const (
	LifecyclePolicyActive   LifecyclePolicyStatus = "ACTIVE"
	LifecyclePolicyDisabled LifecyclePolicyStatus = "DISABLED"
)

type LifecycleJobType string

const (
	LifecycleJobMigrate   LifecycleJobType = "MIGRATE"
	LifecycleJobReplicate LifecycleJobType = "REPLICATE"
	LifecycleJobArchive   LifecycleJobType = "ARCHIVE"
	LifecycleJobGC        LifecycleJobType = "GC"
)

type LifecycleJobStatus string

const (
	LifecycleJobPending   LifecycleJobStatus = "PENDING"
	LifecycleJobRunning   LifecycleJobStatus = "RUNNING"
	LifecycleJobCompleted LifecycleJobStatus = "COMPLETED"
	LifecycleJobFailed    LifecycleJobStatus = "FAILED"
	LifecycleJobCancelled LifecycleJobStatus = "CANCELLED"
)

type LifecyclePolicy struct {
	ID             uuid.UUID
	OrganizationID uuid.UUID
	Name           string
	Version        int64
	Rules          map[string]any
	Status         LifecyclePolicyStatus
}

type LifecyclePolicyInput struct {
	ID             uuid.UUID
	OrganizationID uuid.UUID
	Name           string
	Version        int64
	Rules          map[string]any
	Status         LifecyclePolicyStatus
}

func NewLifecyclePolicy(input LifecyclePolicyInput) (*LifecyclePolicy, error) {
	if input.ID == uuid.Nil || input.OrganizationID == uuid.Nil {
		return nil, ErrInvalidLifecyclePolicy
	}
	name := strings.TrimSpace(input.Name)
	if name == "" {
		return nil, ErrInvalidLifecyclePolicy
	}
	version := input.Version
	if version <= 0 {
		version = 1
	}
	status := input.Status
	if status == "" {
		status = LifecyclePolicyActive
	}
	rules := input.Rules
	if rules == nil {
		rules = map[string]any{}
	}
	return &LifecyclePolicy{
		ID:             input.ID,
		OrganizationID: input.OrganizationID,
		Name:           name,
		Version:        version,
		Rules:          rules,
		Status:         status,
	}, nil
}

type LifecycleJob struct {
	ID             uuid.UUID
	OrganizationID uuid.UUID
	PolicyID       *uuid.UUID
	JobType        LifecycleJobType
	Status         LifecycleJobStatus
	IdempotencyKey string
	LeaseOwner     string
	LeaseExpiresAt *time.Time
	Checkpoint     map[string]any
	AttemptCount   int
	MaxAttempts    int
	LastError      string
}

type LifecycleJobInput struct {
	ID             uuid.UUID
	OrganizationID uuid.UUID
	PolicyID       *uuid.UUID
	JobType        LifecycleJobType
	Status         LifecycleJobStatus
	IdempotencyKey string
	LeaseOwner     string
	LeaseExpiresAt *time.Time
	Checkpoint     map[string]any
	AttemptCount   int
	MaxAttempts    int
	LastError      string
}

func NewLifecycleJob(input LifecycleJobInput) (*LifecycleJob, error) {
	if input.ID == uuid.Nil || input.OrganizationID == uuid.Nil {
		return nil, ErrInvalidLifecycleJob
	}
	idempotencyKey := strings.TrimSpace(input.IdempotencyKey)
	if idempotencyKey == "" {
		return nil, ErrInvalidLifecycleJob
	}
	switch input.JobType {
	case LifecycleJobMigrate, LifecycleJobReplicate, LifecycleJobArchive, LifecycleJobGC:
	default:
		return nil, ErrInvalidLifecycleJob
	}
	status := input.Status
	if status == "" {
		status = LifecycleJobPending
	}
	maxAttempts := input.MaxAttempts
	if maxAttempts <= 0 {
		maxAttempts = 10
	}
	if input.AttemptCount < 0 {
		return nil, ErrInvalidLifecycleJob
	}
	checkpoint := input.Checkpoint
	if checkpoint == nil {
		checkpoint = map[string]any{}
	}
	return &LifecycleJob{
		ID:             input.ID,
		OrganizationID: input.OrganizationID,
		PolicyID:       input.PolicyID,
		JobType:        input.JobType,
		Status:         status,
		IdempotencyKey: idempotencyKey,
		LeaseOwner:     strings.TrimSpace(input.LeaseOwner),
		LeaseExpiresAt: input.LeaseExpiresAt,
		Checkpoint:     checkpoint,
		AttemptCount:   input.AttemptCount,
		MaxAttempts:    maxAttempts,
		LastError:      strings.TrimSpace(input.LastError),
	}, nil
}

type Mount struct {
	ID          uuid.UUID
	NamespaceID uuid.UUID
	MountPath   string
	PoolID      uuid.UUID
	AccessMode  MountAccessMode
}

type MountInput struct {
	ID          uuid.UUID
	NamespaceID uuid.UUID
	MountPath   string
	PoolID      uuid.UUID
	AccessMode  MountAccessMode
}

var ErrInvalidMount = errors.New("invalid mount")

func NewMount(input MountInput) (*Mount, error) {
	if input.ID == uuid.Nil || input.NamespaceID == uuid.Nil || input.PoolID == uuid.Nil {
		return nil, ErrInvalidMount
	}
	mountPath := strings.TrimSpace(input.MountPath)
	if mountPath == "" || !strings.HasPrefix(mountPath, "/") {
		return nil, ErrInvalidMount
	}
	switch input.AccessMode {
	case MountAccessReadOnly, MountAccessReadWrite:
	default:
		return nil, ErrInvalidMount
	}
	return &Mount{
		ID:          input.ID,
		NamespaceID: input.NamespaceID,
		MountPath:   mountPath,
		PoolID:      input.PoolID,
		AccessMode:  input.AccessMode,
	}, nil
}
