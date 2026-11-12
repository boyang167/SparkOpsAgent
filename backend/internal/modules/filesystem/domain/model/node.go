package model

import (
	"strings"

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
}

func NewFile(namespaceID, ownerID uuid.UUID, name string, parentID uuid.UUID) (*Node, error) {
	return newNode(namespaceID, ownerID, name, parentID, NodeTypeFile)
}

func NewDirectory(namespaceID, ownerID uuid.UUID, name string, parentID uuid.UUID) (*Node, error) {
	return newNode(namespaceID, ownerID, name, parentID, NodeTypeDirectory)
}

func newNode(namespaceID, ownerID uuid.UUID, name string, parentID uuid.UUID, nodeType NodeType) (*Node, error) {
	node := &Node{
		ID:          uuid.New(),
		NamespaceID: namespaceID,
		ParentID:    &parentID,
		Type:        nodeType,
		State:       NodeStateActive,
		OwnerID:     ownerID,
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
