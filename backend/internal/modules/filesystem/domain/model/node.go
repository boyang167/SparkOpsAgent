package model

import (
	"strings"
	"time"

	"github.com/google/uuid"
	"golang.org/x/text/unicode/norm"
)

type Node struct {
	ID                uuid.UUID
	NamespaceID       uuid.UUID
	ParentID          *uuid.UUID
	Type              NodeType
	Name              string
	NormalizedName    string
	CurrentRevisionID *uuid.UUID
	State             NodeState
	OwnerID           uuid.UUID
	Classification    TransferClassification
	DeletedAt         *time.Time
	DeletedBy         *uuid.UUID
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

func NewFile(namespaceID, ownerID uuid.UUID, name string, parentID uuid.UUID) (*Node, error) {
	return newNode(namespaceID, ownerID, name, parentID, NodeTypeFile)
}

func NewDirectory(namespaceID, ownerID uuid.UUID, name string, parentID uuid.UUID) (*Node, error) {
	return newNode(namespaceID, ownerID, name, parentID, NodeTypeDirectory)
}

func NewRootDirectory(namespaceID, ownerID uuid.UUID, name string) (*Node, error) {
	node, err := newNode(namespaceID, ownerID, name, uuid.Nil, NodeTypeDirectory)
	if err != nil {
		return nil, err
	}
	node.ParentID = nil
	return node, nil
}

func newNode(namespaceID, ownerID uuid.UUID, name string, parentID uuid.UUID, nodeType NodeType) (*Node, error) {
	node := &Node{
		ID:             uuid.New(),
		NamespaceID:    namespaceID,
		ParentID:       &parentID,
		Type:           nodeType,
		State:          NodeStateActive,
		OwnerID:        ownerID,
		Classification: TransferClassificationInternal,
	}
	if err := node.Rename(name); err != nil {
		return nil, err
	}
	return node, nil
}

func (n *Node) Rename(name string) error {
	normalized := strings.TrimSpace(norm.NFC.String(name))
	if normalized == "" || strings.ContainsAny(normalized, `/\`) {
		return ErrInvalidNodeName
	}
	n.Name = normalized
	n.NormalizedName = strings.ToLower(normalized)
	return nil
}
