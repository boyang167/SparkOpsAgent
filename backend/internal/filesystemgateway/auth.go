package filesystemgateway

import (
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

const (
	claimSessionID  = "session_id"
	claimFileID     = "file_id"
	claimResourceID = "resource_id"
	claimAction     = "action"
	claimNonce      = "nonce"
)

type CapabilityClaims struct {
	SessionID  string `json:"session_id,omitempty"`
	FileID     string `json:"file_id,omitempty"`
	ResourceID string `json:"resource_id,omitempty"`
	Action     string `json:"action"`
	Nonce      string `json:"nonce"`
	jwt.RegisteredClaims
}

type NonceStore interface {
	Consume(nonce string, expiresAt time.Time) error
	Release(nonce string)
}

type MemoryNonceStore struct {
	mu   sync.Mutex
	seen map[string]time.Time
}

func NewMemoryNonceStore() *MemoryNonceStore {
	return &MemoryNonceStore{seen: make(map[string]time.Time)}
}

func (s *MemoryNonceStore) Consume(nonce string, expiresAt time.Time) error {
	if nonce == "" {
		return ErrInvalidCapability
	}
	now := time.Now()
	if !expiresAt.After(now) {
		return ErrCapabilityExpired
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.purgeLocked(now)
	if _, exists := s.seen[nonce]; exists {
		return ErrCapabilityReplay
	}
	s.seen[nonce] = expiresAt
	return nil
}

func (s *MemoryNonceStore) Release(nonce string) {
	s.mu.Lock()
	delete(s.seen, nonce)
	s.mu.Unlock()
}

func (s *MemoryNonceStore) purgeLocked(now time.Time) {
	for nonce, expiresAt := range s.seen {
		if !expiresAt.After(now) {
			delete(s.seen, nonce)
		}
	}
}

type CapabilityAuth struct {
	secret   []byte
	audience string
	nonces   NonceStore
}

func NewCapabilityAuth(opts Options) (*CapabilityAuth, error) {
	if opts.CapabilitySecret == "" {
		return nil, fmt.Errorf("%w: empty capability secret", ErrInvalidGatewayConfig)
	}
	if opts.CapabilityAudience == "" {
		return nil, fmt.Errorf("%w: empty capability audience", ErrInvalidGatewayConfig)
	}
	nonces := opts.NonceStore
	if nonces == nil {
		nonces = NewMemoryNonceStore()
	}
	return &CapabilityAuth{
		secret:   []byte(opts.CapabilitySecret),
		audience: opts.CapabilityAudience,
		nonces:   nonces,
	}, nil
}

func (a *CapabilityAuth) VerifyUpload(tokenString, sessionID string) (*CapabilityClaims, error) {
	claims, err := a.parse(tokenString)
	if err != nil {
		return nil, err
	}
	if claims.Action != ActionTransferWrite {
		return nil, ErrForbidden
	}
	if claims.SessionID == "" || claims.SessionID != sessionID {
		return nil, ErrForbidden
	}
	if err := a.consumeNonce(claims); err != nil {
		return nil, err
	}
	return claims, nil
}

func (a *CapabilityAuth) Release(claims *CapabilityClaims) {
	if claims != nil {
		a.nonces.Release(claims.Nonce)
	}
}

func (a *CapabilityAuth) VerifyDownload(tokenString, fileID string) (*CapabilityClaims, error) {
	claims, err := a.parse(tokenString)
	if err != nil {
		return nil, err
	}
	if claims.Action != ActionTransferRead {
		return nil, ErrForbidden
	}
	resourceID := claims.FileID
	if resourceID == "" {
		resourceID = claims.ResourceID
	}
	if resourceID == "" || resourceID != fileID {
		return nil, ErrForbidden
	}
	if err := a.consumeNonce(claims); err != nil {
		return nil, err
	}
	return claims, nil
}

func (a *CapabilityAuth) parse(tokenString string) (*CapabilityClaims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &CapabilityClaims{}, func(t *jwt.Token) (interface{}, error) {
		if t.Method != jwt.SigningMethodHS256 {
			return nil, errors.New("unexpected signing method")
		}
		return a.secret, nil
	})
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidCapability, err)
	}
	claims, ok := token.Claims.(*CapabilityClaims)
	if !ok || !token.Valid {
		return nil, ErrInvalidCapability
	}
	if !audienceMatches(claims.Audience, a.audience) {
		return nil, ErrForbidden
	}
	if claims.ExpiresAt == nil || !claims.ExpiresAt.After(time.Now()) {
		return nil, ErrCapabilityExpired
	}
	if claims.Nonce == "" {
		return nil, ErrInvalidCapability
	}
	return claims, nil
}

func (a *CapabilityAuth) consumeNonce(claims *CapabilityClaims) error {
	expiresAt := time.Now().Add(time.Hour)
	if claims.ExpiresAt != nil {
		expiresAt = claims.ExpiresAt.Time
	}
	return a.nonces.Consume(claims.Nonce, expiresAt)
}

func audienceMatches(actual jwt.ClaimStrings, expected string) bool {
	for _, aud := range actual {
		if aud == expected {
			return true
		}
	}
	return false
}
