package storage

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
)

var (
	ErrNotFound           = errors.New("storage: object not found")
	ErrObjectConflict     = errors.New("storage: object conflict")
	ErrSizeMismatch       = errors.New("storage: size mismatch")
	ErrChecksumMismatch   = errors.New("storage: checksum mismatch")
	ErrInvalidChecksum    = errors.New("storage: invalid checksum")
	ErrInvalidObjectKey   = errors.New("storage: invalid object key")
	ErrObjectKeyCollision = errors.New("storage: object key collision")
	ErrCorruptObject      = errors.New("storage: corrupt object")
	ErrInvalidRoot        = errors.New("storage: invalid root")
	ErrInvalidRange       = errors.New("storage: invalid range")
	ErrOutcomeUnknown     = errors.New("storage: outcome unknown")
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

func SHA256(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}
