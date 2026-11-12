package domain

import "errors"

var (
	ErrNodeNameConflict     = errors.New("filesystem node name conflict")
	ErrNodeNotFound         = errors.New("filesystem node not found")
	ErrNodeIDConflict       = errors.New("filesystem node id conflict")
	ErrNamespaceNotFound    = errors.New("filesystem namespace not found")
	ErrRevisionNotFound     = errors.New("filesystem revision not found")
	ErrRevisionConflict     = errors.New("filesystem revision conflict")
	ErrRevisionNodeMismatch = errors.New("filesystem revision belongs to another node")
	ErrParentNotFound       = errors.New("filesystem parent not found")
	ErrParentNotDirectory   = errors.New("filesystem parent is not a directory")
	ErrNamespaceMismatch    = errors.New("filesystem namespace mismatch")
	ErrBlobConflict         = errors.New("filesystem blob conflict")
	ErrReplicaConflict      = errors.New("filesystem replica conflict")
	ErrPersistence          = errors.New("filesystem persistence failure")
)
