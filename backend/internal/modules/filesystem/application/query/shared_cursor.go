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

	"github.com/google/uuid"
	"golang.org/x/text/unicode/norm"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/domain"
)

const (
	sharedCursorVersion = byte(1)
	sharedCursorDomain  = "filesystem-shared-cursor-v1"
	sharedCursorSource  = "shared-with-me"
)

type SharedCursorContext struct {
	OrganizationID uuid.UUID
	ActorID        uuid.UUID
	WorkspaceID    uuid.UUID
	NamespaceID    uuid.UUID
	Query          string
}

type SharedCursorCodec struct {
	aead        cipher.AEAD
	nonceReader io.Reader
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

func sharedCursorAAD(context SharedCursorContext) []byte {
	aad, _ := json.Marshal(struct {
		Version        byte      `json:"version"`
		Source         string    `json:"source"`
		OrganizationID uuid.UUID `json:"organization_id"`
		ActorID        uuid.UUID `json:"actor_id"`
		WorkspaceID    uuid.UUID `json:"workspace_id"`
		NamespaceID    uuid.UUID `json:"namespace_id"`
		Query          string    `json:"query"`
	}{
		Version:        sharedCursorVersion,
		Source:         sharedCursorSource,
		OrganizationID: context.OrganizationID,
		ActorID:        context.ActorID,
		WorkspaceID:    context.WorkspaceID,
		NamespaceID:    context.NamespaceID,
		Query:          strings.ToLower(strings.TrimSpace(norm.NFC.String(context.Query))),
	})
	return aad
}
