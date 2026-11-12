package model

import "github.com/google/uuid"

type FileRevision struct {
	ID         uuid.UUID
	NodeID     uuid.UUID
	RevisionNo int
	BlobID     uuid.UUID
	Size       int64
	MIMEType   string
	CreatedBy  uuid.UUID
}

func NewRevision(nodeID uuid.UUID, revisionNo int, blobID uuid.UUID, size int64, mimeType string, createdBy uuid.UUID) FileRevision {
	return FileRevision{
		ID:         uuid.New(),
		NodeID:     nodeID,
		RevisionNo: revisionNo,
		BlobID:     blobID,
		Size:       size,
		MIMEType:   mimeType,
		CreatedBy:  createdBy,
	}
}
