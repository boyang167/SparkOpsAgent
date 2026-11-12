package domain

import "errors"

var (
	ErrNodeNameConflict  = errors.New("filesystem node name conflict")
	ErrNodeNotFound      = errors.New("filesystem node not found")
	ErrNamespaceNotFound = errors.New("filesystem namespace not found")
)
