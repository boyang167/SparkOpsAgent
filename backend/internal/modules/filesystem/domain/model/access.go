package model

import (
	"time"

	"github.com/google/uuid"
)

const (
	ExportStatusPending  = "PENDING"
	ExportStatusImported = "IMPORTED"
)

type CapabilityRecord struct {
	ID             uuid.UUID
	TokenHash      string
	OrganizationID uuid.UUID
	WorkspaceID    uuid.UUID
	ActorID        uuid.UUID
	AgentID        *uuid.UUID
	RunID          uuid.UUID
	ResourceIDs    []uuid.UUID
	Actions        []string
	Audience       string
	ExpiresAt      time.Time
	RevokedAt      *time.Time
}

type ExportTask struct {
	ID                   uuid.UUID
	SourceOrganizationID uuid.UUID
	TargetOrganizationID uuid.UUID
	SourceWorkspaceID    uuid.UUID
	TargetWorkspaceID    uuid.UUID
	SourceNodeID         uuid.UUID
	TargetNodeID         *uuid.UUID
	Status               string
	ActorID              uuid.UUID
	Classification       TransferClassification
}
