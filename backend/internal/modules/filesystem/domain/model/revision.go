package model

import (
	"errors"
	"mime"
	"strings"

	"github.com/google/uuid"
)

var ErrInvalidRevision = errors.New("invalid file revision")

type FileRevision struct {
	id         uuid.UUID
	nodeID     uuid.UUID
	blobID     uuid.UUID
	revisionNo int
	size       int64
	mimeType   string
	createdBy  uuid.UUID
}

func NewRevision(nodeID uuid.UUID, revisionNo int, blobID uuid.UUID, size int64, mimeType string, createdBy uuid.UUID) (*FileRevision, error) {
	if revisionNo <= 0 || size < 0 {
		return nil, ErrInvalidRevision
	}
	mimeType = strings.TrimSpace(mimeType)
	if mimeType == "" {
		return nil, ErrInvalidRevision
	}
	if _, _, err := mime.ParseMediaType(mimeType); err != nil {
		return nil, ErrInvalidRevision
	}
	return &FileRevision{
		id:         uuid.New(),
		nodeID:     nodeID,
		blobID:     blobID,
		revisionNo: revisionNo,
		size:       size,
		mimeType:   mimeType,
		createdBy:  createdBy,
	}, nil
}

func (r *FileRevision) ID() uuid.UUID {
	return r.id
}

func (r *FileRevision) NodeID() uuid.UUID {
	return r.nodeID
}

func (r *FileRevision) BlobID() uuid.UUID {
	return r.blobID
}

func (r *FileRevision) RevisionNo() int {
	return r.revisionNo
}

func (r *FileRevision) Size() int64 {
	return r.size
}

func (r *FileRevision) MIMEType() string {
	return r.mimeType
}

func (r *FileRevision) CreatedBy() uuid.UUID {
	return r.createdBy
}
