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

func NewFile(namespaceID, ownerID uuid.UUID, name string, parentID uuid.UUID) *Node {
	node := &Node{
		ID:          uuid.New(),
		NamespaceID: namespaceID,
		ParentID:    &parentID,
		Type:        NodeTypeFile,
		State:       NodeStateActive,
		OwnerID:     ownerID,
	}
	_ = node.Rename(name)
	return node
}

func (n *Node) Rename(name string) error {
	normalized := strings.TrimSpace(norm.NFC.String(name))
	if normalized == "" || strings.ContainsAny(normalized, `/\`) {
		return ErrInvalidNodeName
	}
	n.Name = name
	n.NormalizedName = strings.ToLower(normalized)
	return nil
}
