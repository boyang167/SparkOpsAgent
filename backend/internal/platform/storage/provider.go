package storage

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"time"
)

var (
	ErrNotFound            = errors.New("storage: object not found")
	ErrObjectConflict      = errors.New("storage: object conflict")
	ErrSizeMismatch        = errors.New("storage: size mismatch")
	ErrChecksumMismatch    = errors.New("storage: checksum mismatch")
	ErrInvalidChecksum     = errors.New("storage: invalid checksum")
	ErrInvalidObjectKey    = errors.New("storage: invalid object key")
	ErrObjectKeyCollision  = errors.New("storage: object key collision")
	ErrCorruptObject       = errors.New("storage: corrupt object")
	ErrInvalidRoot         = errors.New("storage: invalid root")
	ErrInvalidRange        = errors.New("storage: invalid range")
	ErrOutcomeUnknown      = errors.New("storage: outcome unknown")
	ErrUnsupportedPlatform = errors.New("storage: unsupported platform")
	ErrMultipartNotFound   = errors.New("storage: multipart upload not found")
	ErrInvalidPart         = errors.New("storage: invalid multipart part")
)

type Provider interface {
	ID() string
	Capabilities() Capabilities
	Put(context.Context, PutRequest) (ObjectInfo, error)
	Get(context.Context, GetRequest) (io.ReadCloser, ObjectInfo, error)
	Stat(context.Context, string) (ObjectInfo, error)
	Delete(context.Context, string) error
	Copy(context.Context, string, string) (ObjectInfo, error)
}

type PutRequest struct {
	ObjectKey string
	Body      io.Reader
	Size      int64
	SHA256    string
}

type GetRequest struct {
	ObjectKey string
	Offset    int64
	Length    int64
}

type ObjectInfo struct {
	ObjectKey string
	Size      int64
	SHA256    string
}

type MultipartUpload struct {
	UploadID  string
	ObjectKey string
}

type CompletedPart struct {
	PartNumber int
	ETag       string
}

type UploadedPart struct {
	PartNumber int
	ETag       string
	Size       int64
}

type MultipartProvider interface {
	CreateMultipart(ctx context.Context, objectKey string, size int64) (MultipartUpload, error)
	ListParts(ctx context.Context, uploadID string) ([]UploadedPart, error)
	PresignPart(ctx context.Context, uploadID string, partNumber int, expires time.Duration) (string, error)
	CompleteMultipart(ctx context.Context, uploadID string, parts []CompletedPart) (ObjectInfo, error)
	AbortMultipart(ctx context.Context, uploadID string) error
}

type PresignProvider interface {
	PresignGet(ctx context.Context, objectKey string, expires time.Duration) (string, error)
}

func SHA256(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}
