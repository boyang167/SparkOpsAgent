package model

import (
	"time"

	"github.com/google/uuid"
)

type ShareRequestStatus string

const (
	ShareRequestPending   ShareRequestStatus = "PENDING"
	ShareRequestApproved  ShareRequestStatus = "APPROVED"
	ShareRequestRejected  ShareRequestStatus = "REJECTED"
	ShareRequestCancelled ShareRequestStatus = "CANCELLED"
)

type ShareRequest struct {
	ID                uuid.UUID
	OrganizationID    uuid.UUID
	NamespaceID       uuid.UUID
	NodeID            uuid.UUID
	RequestedBy       uuid.UUID
	PrincipalType     PrincipalType
	PrincipalIDs      []uuid.UUID
	Actions           []ResourcePermission
	InheritToChildren bool
	ExpiresAt         *time.Time
	Classification    TransferClassification
	Status            ShareRequestStatus
	ReviewedBy        *uuid.UUID
	ReviewedAt        *time.Time
	RejectionReason   string
	CreatedAt         time.Time
}
