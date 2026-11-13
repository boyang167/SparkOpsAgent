package model

import (
	"errors"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
)

const maxTransferSize int64 = 20 * 1024 * 1024 * 1024

var (
	ErrInvalidTransfer           = errors.New("invalid transfer session")
	ErrInvalidTransferName       = errors.New("invalid transfer name")
	ErrInvalidTransferSize       = errors.New("invalid transfer size")
	ErrInvalidTransferChecksum   = errors.New("invalid transfer checksum")
	ErrInvalidTransferProtocol   = errors.New("unsupported transfer protocol")
	ErrInvalidTransferExpiry     = errors.New("transfer expiry must be in the future")
	ErrInvalidTransferTransition = errors.New("invalid transfer transition")
	ErrTransferVersionConflict   = errors.New("transfer version conflict")
	ErrInvalidTransferPart       = errors.New("invalid transfer part")
)

type TransferStatus string

const (
	TransferInitiated  TransferStatus = "INITIATED"
	TransferUploading  TransferStatus = "UPLOADING"
	TransferPaused     TransferStatus = "PAUSED"
	TransferVerifying  TransferStatus = "VERIFYING"
	TransferScanning   TransferStatus = "SCANNING"
	TransferCommitting TransferStatus = "COMMITTING"
	TransferCompleted  TransferStatus = "COMPLETED"
	TransferFailed     TransferStatus = "FAILED"
	TransferRejected   TransferStatus = "REJECTED"
	TransferAborted    TransferStatus = "ABORTED"
)

type TransferProtocol string

const (
	TransferProtocolMultipart TransferProtocol = "MULTIPART"
	TransferProtocolDirect    TransferProtocol = "DIRECT"
)

type TransferClassification string

const (
	TransferClassificationGeneral    TransferClassification = "GENERAL"
	TransferClassificationRestricted TransferClassification = "RESTRICTED"
)

type TransferSessionInput struct {
	ID                 uuid.UUID
	WorkspaceID        uuid.UUID
	NamespaceID        uuid.UUID
	TargetParentID     *uuid.UUID
	TargetName         string
	CreatedBy          uuid.UUID
	Protocol           TransferProtocol
	ExpectedSize       int64
	ExpectedSHA256     string
	IdempotencyKey     string
	TemporaryObjectKey string
	ExpiresAt          time.Time
	ProviderID         *uuid.UUID
	Classification     TransferClassification
}

type TransferSession struct {
	id                 uuid.UUID
	workspaceID        uuid.UUID
	namespaceID        uuid.UUID
	targetParentID     *uuid.UUID
	targetName         string
	createdBy          uuid.UUID
	protocol           TransferProtocol
	expectedSize       int64
	expectedSHA256     string
	actualSize         *int64
	actualSHA256       string
	providerID         *uuid.UUID
	temporaryObjectKey string
	status             TransferStatus
	idempotencyKey     string
	expiresAt          time.Time
	classification     TransferClassification
	scanResult         *TransferScanResult
	errorCode          string
	version            int64
}

type TransferScanResult struct {
	Clean      bool
	ThreatName string
	Engine     string
	Signature  string
}

var transferNamePattern = regexp.MustCompile(`^[^/\\\x00]+$`)

func NewTransferSession(input TransferSessionInput) (*TransferSession, error) {
	if input.ID == uuid.Nil || input.WorkspaceID == uuid.Nil || input.NamespaceID == uuid.Nil ||
		input.CreatedBy == uuid.Nil || strings.TrimSpace(input.IdempotencyKey) == "" ||
		strings.TrimSpace(input.TemporaryObjectKey) == "" {
		return nil, ErrInvalidTransfer
	}
	input.TargetName = strings.TrimSpace(input.TargetName)
	if input.TargetName == "" || !transferNamePattern.MatchString(input.TargetName) {
		return nil, ErrInvalidTransferName
	}
	if input.ExpectedSize < 0 || input.ExpectedSize > maxTransferSize {
		return nil, ErrInvalidTransferSize
	}
	if input.ExpectedSHA256 != "" && !isSHA256(input.ExpectedSHA256) {
		return nil, ErrInvalidTransferChecksum
	}
	if input.Protocol != TransferProtocolDirect && input.Protocol != TransferProtocolMultipart {
		return nil, ErrInvalidTransferProtocol
	}
	if !input.ExpiresAt.After(time.Now()) {
		return nil, ErrInvalidTransferExpiry
	}
	if input.Classification == "" {
		input.Classification = TransferClassificationGeneral
	}
	if input.Classification != TransferClassificationGeneral && input.Classification != TransferClassificationRestricted {
		return nil, ErrInvalidTransfer
	}
	var targetParentID, providerID *uuid.UUID
	if input.TargetParentID != nil {
		value := *input.TargetParentID
		targetParentID = &value
	}
	if input.ProviderID != nil {
		value := *input.ProviderID
		providerID = &value
	}
	return &TransferSession{
		id: input.ID, workspaceID: input.WorkspaceID, namespaceID: input.NamespaceID,
		targetParentID: targetParentID, targetName: input.TargetName, createdBy: input.CreatedBy,
		protocol: input.Protocol, expectedSize: input.ExpectedSize, expectedSHA256: input.ExpectedSHA256,
		providerID: providerID, temporaryObjectKey: input.TemporaryObjectKey,
		status: TransferInitiated, idempotencyKey: input.IdempotencyKey, expiresAt: input.ExpiresAt,
		classification: input.Classification, version: 1,
	}, nil
}

func isSHA256(value string) bool {
	if len(value) != 64 {
		return false
	}
	for _, c := range value {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')) {
			return false
		}
	}
	return true
}

func (s *TransferSession) ID() uuid.UUID          { return s.id }
func (s *TransferSession) WorkspaceID() uuid.UUID { return s.workspaceID }
func (s *TransferSession) NamespaceID() uuid.UUID { return s.namespaceID }
func (s *TransferSession) TargetParentID() *uuid.UUID {
	if s.targetParentID == nil {
		return nil
	}
	value := *s.targetParentID
	return &value
}
func (s *TransferSession) TargetName() string         { return s.targetName }
func (s *TransferSession) CreatedBy() uuid.UUID       { return s.createdBy }
func (s *TransferSession) Protocol() TransferProtocol { return s.protocol }
func (s *TransferSession) ExpectedSize() int64        { return s.expectedSize }
func (s *TransferSession) ExpectedSHA256() string     { return s.expectedSHA256 }
func (s *TransferSession) ActualSize() int64 {
	if s.actualSize == nil {
		return 0
	}
	return *s.actualSize
}
func (s *TransferSession) ActualSHA256() string { return s.actualSHA256 }
func (s *TransferSession) ProviderID() *uuid.UUID {
	if s.providerID == nil {
		return nil
	}
	value := *s.providerID
	return &value
}
func (s *TransferSession) TemporaryObjectKey() string             { return s.temporaryObjectKey }
func (s *TransferSession) Status() TransferStatus                 { return s.status }
func (s *TransferSession) IdempotencyKey() string                 { return s.idempotencyKey }
func (s *TransferSession) ExpiresAt() time.Time                   { return s.expiresAt }
func (s *TransferSession) Classification() TransferClassification { return s.classification }
func (s *TransferSession) ScanResult() *TransferScanResult {
	if s.scanResult == nil {
		return nil
	}
	value := *s.scanResult
	return &value
}
func (s *TransferSession) ErrorCode() string { return s.errorCode }
func (s *TransferSession) Version() int64    { return s.version }

func (s *TransferSession) transition(expected []TransferStatus, next TransferStatus, versions []int64) error {
	if len(versions) == 1 && s.version != versions[0] {
		return ErrTransferVersionConflict
	}
	for _, current := range expected {
		if s.status == current {
			s.status = next
			s.version++
			return nil
		}
	}
	return ErrInvalidTransferTransition
}

func (s *TransferSession) StartUpload(expectedVersion ...int64) error {
	return s.transition([]TransferStatus{TransferInitiated, TransferPaused}, TransferUploading, expectedVersion)
}
func (s *TransferSession) Pause(expectedVersion ...int64) error {
	return s.transition([]TransferStatus{TransferUploading}, TransferPaused, expectedVersion)
}
func (s *TransferSession) Resume(expectedVersion ...int64) error {
	return s.StartUpload(expectedVersion...)
}
func (s *TransferSession) StartVerify(expectedVersion ...int64) error {
	return s.transition([]TransferStatus{TransferUploading}, TransferVerifying, expectedVersion)
}
func (s *TransferSession) StartScan(expectedVersion ...int64) error {
	return s.transition([]TransferStatus{TransferVerifying}, TransferScanning, expectedVersion)
}
func (s *TransferSession) StartCommit(expectedVersion ...int64) error {
	return s.transition([]TransferStatus{TransferScanning}, TransferCommitting, expectedVersion)
}
func (s *TransferSession) Complete(expectedVersion ...int64) error {
	return s.transition([]TransferStatus{TransferCommitting}, TransferCompleted, expectedVersion)
}
func (s *TransferSession) Fail(code string, expectedVersion ...int64) error {
	if strings.TrimSpace(code) == "" {
		return ErrInvalidTransfer
	}
	if err := s.transition([]TransferStatus{TransferInitiated, TransferUploading, TransferPaused, TransferVerifying, TransferScanning, TransferCommitting}, TransferFailed, expectedVersion); err != nil {
		return err
	}
	s.errorCode = code
	return nil
}
func (s *TransferSession) Reject(code string, expectedVersion ...int64) error {
	if strings.TrimSpace(code) == "" {
		return ErrInvalidTransfer
	}
	if err := s.transition([]TransferStatus{TransferInitiated, TransferUploading, TransferPaused, TransferVerifying, TransferScanning, TransferCommitting}, TransferRejected, expectedVersion); err != nil {
		return err
	}
	s.errorCode = code
	return nil
}
func (s *TransferSession) Abort(expectedVersion ...int64) error {
	return s.transition([]TransferStatus{TransferInitiated, TransferUploading, TransferPaused, TransferVerifying}, TransferAborted, expectedVersion)
}

func (s *TransferSession) SetActual(size int64, checksum string, expectedVersion int64) error {
	if size < 0 || size > maxTransferSize || !isSHA256(checksum) || s.version != expectedVersion {
		if s.version != expectedVersion {
			return ErrTransferVersionConflict
		}
		return ErrInvalidTransfer
	}
	s.actualSize, s.actualSHA256 = &size, checksum
	s.version++
	return nil
}

func (s *TransferSession) SetProvider(providerID uuid.UUID, expectedVersion int64) error {
	if providerID == uuid.Nil {
		return ErrInvalidTransfer
	}
	if s.version != expectedVersion {
		return ErrTransferVersionConflict
	}
	s.providerID = &providerID
	s.version++
	return nil
}

func (s *TransferSession) SetScanResult(result TransferScanResult, expectedVersion int64) error {
	if s.version != expectedVersion {
		return ErrTransferVersionConflict
	}
	s.scanResult = &result
	s.version++
	return nil
}

type TransferPartInput struct {
	ID         uuid.UUID
	SessionID  uuid.UUID
	PartNumber int
	Offset     int64
	Size       int64
	Checksum   string
	ETag       string
}

type TransferPart struct {
	id, sessionID  uuid.UUID
	partNumber     int
	offset, size   int64
	checksum, etag string
}

func NewTransferPart(input TransferPartInput) (*TransferPart, error) {
	if input.ID == uuid.Nil || input.SessionID == uuid.Nil || input.PartNumber <= 0 ||
		input.Offset < 0 || input.Size < 0 || (input.Checksum != "" && !isSHA256(input.Checksum)) {
		return nil, ErrInvalidTransferPart
	}
	return &TransferPart{id: input.ID, sessionID: input.SessionID, partNumber: input.PartNumber,
		offset: input.Offset, size: input.Size, checksum: input.Checksum, etag: input.ETag}, nil
}
func (p *TransferPart) ID() uuid.UUID        { return p.id }
func (p *TransferPart) SessionID() uuid.UUID { return p.sessionID }
func (p *TransferPart) PartNumber() int      { return p.partNumber }
func (p *TransferPart) Offset() int64        { return p.offset }
func (p *TransferPart) Size() int64          { return p.size }
func (p *TransferPart) Checksum() string     { return p.checksum }
func (p *TransferPart) ETag() string         { return p.etag }
