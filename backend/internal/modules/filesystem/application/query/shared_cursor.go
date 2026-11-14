package query

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"time"

	"github.com/google/uuid"
	"golang.org/x/text/unicode/norm"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/domain"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/domain/model"
)

const (
	sharedCursorVersion    = byte(1)
	sharedCursorDomain     = "filesystem-shared-cursor-v1"
	sharedCursorSource     = "shared-with-me"
	childrenCursorSource   = "children"
	governanceCursorSource = "governance-metadata"
)

type SharedCursorContext struct {
	OrganizationID uuid.UUID
	ActorID        uuid.UUID
	WorkspaceID    uuid.UUID
	NamespaceID    uuid.UUID
	SourceType     model.PrincipalType
	Query          string
}

type SharedCursorCodec struct {
	aead        cipher.AEAD
	nonceReader io.Reader
}

type ChildrenCursorContext struct {
	NamespaceID uuid.UUID
	ParentID    uuid.UUID
	ActorID     uuid.UUID
	WorkspaceID uuid.UUID
	NodeType    *model.NodeType
	Query       string
}

type GovernanceCursorContext struct {
	OrganizationID uuid.UUID
	NodeID         uuid.UUID
	Kind           string
}

type GovernanceCursorPosition struct {
	CreatedAt time.Time `json:"created_at"`
	ID        uuid.UUID `json:"id"`
}

func NewSharedCursorCodec(secret string, nonceReader io.Reader) (*SharedCursorCodec, error) {
	if strings.TrimSpace(secret) == "" {
		return nil, errors.New("shared cursor secret is required")
	}
	digest := sha256.New()
	_, _ = digest.Write([]byte(sharedCursorDomain))
	_, _ = digest.Write([]byte{0})
	_, _ = digest.Write([]byte(secret))
	return newSharedCursorCodec(digest.Sum(nil), nonceReader)
}

func newSharedCursorCodec(key []byte, nonceReader io.Reader) (*SharedCursorCodec, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	if nonceReader == nil {
		nonceReader = rand.Reader
	}
	return &SharedCursorCodec{aead: aead, nonceReader: nonceReader}, nil
}

func (c *SharedCursorCodec) Encode(after string, context SharedCursorContext) (string, error) {
	if c == nil || c.aead == nil || c.nonceReader == nil {
		return "", domain.ErrPersistence
	}
	position, err := uuid.Parse(after)
	if err != nil || position == uuid.Nil {
		return "", domain.ErrPersistence
	}
	nonce := make([]byte, c.aead.NonceSize())
	if _, err := io.ReadFull(c.nonceReader, nonce); err != nil {
		return "", domain.ErrPersistence
	}
	ciphertext := c.aead.Seal(nil, nonce, position[:], sharedCursorAAD(context))
	token := make([]byte, 1+len(nonce)+len(ciphertext))
	token[0] = sharedCursorVersion
	copy(token[1:], nonce)
	copy(token[1+len(nonce):], ciphertext)
	return base64.RawURLEncoding.EncodeToString(token), nil
}

func (c *SharedCursorCodec) Decode(value string, context SharedCursorContext) (string, error) {
	if value == "" {
		return "", nil
	}
	if c == nil || c.aead == nil {
		return "", domain.ErrInvalidGovernanceCursor
	}
	token, err := base64.RawURLEncoding.DecodeString(value)
	minimum := 1 + c.aead.NonceSize() + c.aead.Overhead() + len(uuid.UUID{})
	if err != nil || len(token) != minimum || token[0] != sharedCursorVersion {
		return "", domain.ErrInvalidGovernanceCursor
	}
	nonceEnd := 1 + c.aead.NonceSize()
	plaintext, err := c.aead.Open(nil, token[1:nonceEnd], token[nonceEnd:], sharedCursorAAD(context))
	if err != nil || len(plaintext) != len(uuid.UUID{}) {
		return "", domain.ErrInvalidGovernanceCursor
	}
	position, err := uuid.FromBytes(plaintext)
	if err != nil || position == uuid.Nil {
		return "", domain.ErrInvalidGovernanceCursor
	}
	return position.String(), nil
}

func (c *SharedCursorCodec) EncodeChildren(after string, context ChildrenCursorContext) (string, error) {
	if c == nil || c.aead == nil || c.nonceReader == nil {
		return "", domain.ErrPersistence
	}
	position, err := uuid.Parse(after)
	if err != nil || position == uuid.Nil {
		return "", domain.ErrPersistence
	}
	nonce := make([]byte, c.aead.NonceSize())
	if _, err := io.ReadFull(c.nonceReader, nonce); err != nil {
		return "", domain.ErrPersistence
	}
	ciphertext := c.aead.Seal(nil, nonce, position[:], childrenCursorAAD(context))
	token := make([]byte, 1+len(nonce)+len(ciphertext))
	token[0] = sharedCursorVersion
	copy(token[1:], nonce)
	copy(token[1+len(nonce):], ciphertext)
	return base64.RawURLEncoding.EncodeToString(token), nil
}

func (c *SharedCursorCodec) DecodeChildren(value string, context ChildrenCursorContext) (string, error) {
	if value == "" {
		return "", nil
	}
	if c == nil || c.aead == nil {
		return "", domain.ErrInvalidGovernanceCursor
	}
	token, err := base64.RawURLEncoding.DecodeString(value)
	minimum := 1 + c.aead.NonceSize() + c.aead.Overhead() + len(uuid.UUID{})
	if err != nil || len(token) != minimum || token[0] != sharedCursorVersion {
		return "", domain.ErrInvalidGovernanceCursor
	}
	nonceEnd := 1 + c.aead.NonceSize()
	plaintext, err := c.aead.Open(nil, token[1:nonceEnd], token[nonceEnd:], childrenCursorAAD(context))
	if err != nil || len(plaintext) != len(uuid.UUID{}) {
		return "", domain.ErrInvalidGovernanceCursor
	}
	position, err := uuid.FromBytes(plaintext)
	if err != nil || position == uuid.Nil {
		return "", domain.ErrInvalidGovernanceCursor
	}
	return position.String(), nil
}

func (c *SharedCursorCodec) EncodeGovernance(position GovernanceCursorPosition, context GovernanceCursorContext) (string, error) {
	if c == nil || c.aead == nil || c.nonceReader == nil || position.ID == uuid.Nil {
		return "", domain.ErrPersistence
	}
	plaintext, err := json.Marshal(GovernanceCursorPosition{
		CreatedAt: position.CreatedAt.UTC(), ID: position.ID,
	})
	if err != nil {
		return "", domain.ErrPersistence
	}
	nonce := make([]byte, c.aead.NonceSize())
	if _, err := io.ReadFull(c.nonceReader, nonce); err != nil {
		return "", domain.ErrPersistence
	}
	ciphertext := c.aead.Seal(nil, nonce, plaintext, governanceCursorAAD(context))
	token := make([]byte, 1+len(nonce)+len(ciphertext))
	token[0] = sharedCursorVersion
	copy(token[1:], nonce)
	copy(token[1+len(nonce):], ciphertext)
	return base64.RawURLEncoding.EncodeToString(token), nil
}

func (c *SharedCursorCodec) DecodeGovernance(value string, context GovernanceCursorContext) (GovernanceCursorPosition, error) {
	if value == "" {
		return GovernanceCursorPosition{}, nil
	}
	if c == nil || c.aead == nil {
		return GovernanceCursorPosition{}, domain.ErrInvalidGovernanceCursor
	}
	token, err := base64.RawURLEncoding.DecodeString(value)
	minimum := 1 + c.aead.NonceSize() + c.aead.Overhead() + 2
	if err != nil || len(token) < minimum || token[0] != sharedCursorVersion {
		return GovernanceCursorPosition{}, domain.ErrInvalidGovernanceCursor
	}
	nonceEnd := 1 + c.aead.NonceSize()
	plaintext, err := c.aead.Open(nil, token[1:nonceEnd], token[nonceEnd:], governanceCursorAAD(context))
	if err != nil {
		return GovernanceCursorPosition{}, domain.ErrInvalidGovernanceCursor
	}
	var position GovernanceCursorPosition
	if err := json.Unmarshal(plaintext, &position); err != nil || position.ID == uuid.Nil {
		return GovernanceCursorPosition{}, domain.ErrInvalidGovernanceCursor
	}
	position.CreatedAt = position.CreatedAt.UTC()
	return position, nil
}

func sharedCursorAAD(context SharedCursorContext) []byte {
	aad, _ := json.Marshal(struct {
		Version        byte                `json:"version"`
		Source         string              `json:"source"`
		OrganizationID uuid.UUID           `json:"organization_id"`
		ActorID        uuid.UUID           `json:"actor_id"`
		WorkspaceID    uuid.UUID           `json:"workspace_id"`
		NamespaceID    uuid.UUID           `json:"namespace_id"`
		SourceType     model.PrincipalType `json:"source_type"`
		Query          string              `json:"query"`
	}{
		Version:        sharedCursorVersion,
		Source:         sharedCursorSource,
		OrganizationID: context.OrganizationID,
		ActorID:        context.ActorID,
		WorkspaceID:    context.WorkspaceID,
		NamespaceID:    context.NamespaceID,
		SourceType:     context.SourceType,
		Query:          strings.ToLower(strings.TrimSpace(norm.NFC.String(context.Query))),
	})
	return aad
}

func governanceCursorAAD(context GovernanceCursorContext) []byte {
	aad, _ := json.Marshal(struct {
		Version        byte      `json:"version"`
		Source         string    `json:"source"`
		OrganizationID uuid.UUID `json:"organization_id"`
		NodeID         uuid.UUID `json:"node_id"`
		Kind           string    `json:"kind"`
	}{
		Version: sharedCursorVersion, Source: governanceCursorSource,
		OrganizationID: context.OrganizationID, NodeID: context.NodeID,
		Kind: strings.ToLower(strings.TrimSpace(context.Kind)),
	})
	return aad
}

func childrenCursorAAD(context ChildrenCursorContext) []byte {
	var nodeType model.NodeType
	if context.NodeType != nil {
		nodeType = *context.NodeType
	}
	aad, _ := json.Marshal(struct {
		Version     byte           `json:"version"`
		Source      string         `json:"source"`
		NamespaceID uuid.UUID      `json:"namespace_id"`
		ParentID    uuid.UUID      `json:"parent_id"`
		ActorID     uuid.UUID      `json:"actor_id"`
		WorkspaceID uuid.UUID      `json:"workspace_id"`
		NodeType    model.NodeType `json:"node_type"`
		Query       string         `json:"query"`
	}{
		Version: sharedCursorVersion, Source: childrenCursorSource,
		NamespaceID: context.NamespaceID, ParentID: context.ParentID,
		ActorID: context.ActorID, WorkspaceID: context.WorkspaceID, NodeType: nodeType,
		Query: strings.ToLower(strings.TrimSpace(norm.NFC.String(context.Query))),
	})
	return aad
}
