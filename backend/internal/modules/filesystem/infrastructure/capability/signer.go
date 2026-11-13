package capability

import (
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/domain"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/domain/service"
)

type Signer struct {
	secret   []byte
	audience string
}

func NewSigner(secret, audience string) (*Signer, error) {
	secret = strings.TrimSpace(secret)
	audience = strings.TrimSpace(audience)
	if secret == "" || audience == "" {
		return nil, domain.ErrCapabilityInvalid
	}
	return &Signer{secret: []byte(secret), audience: audience}, nil
}

type tokenClaims struct {
	OrganizationID string   `json:"organization_id"`
	WorkspaceID    string   `json:"workspace_id"`
	ActorID        string   `json:"actor_id"`
	AgentID        string   `json:"agent_id,omitempty"`
	RunID          string   `json:"run_id"`
	ResourceIDs    []string `json:"resource_ids"`
	Actions        []string `json:"actions"`
	Nonce          string   `json:"nonce"`
	jwt.RegisteredClaims
}

func (s *Signer) Sign(claims service.Claims) (string, string, error) {
	if claims.Audience != s.audience || claims.Nonce == "" || claims.RunID == uuid.Nil || len(claims.ResourceIDs) == 0 || !claims.ExpiresAt.After(time.Now()) {
		return "", "", domain.ErrCapabilityInvalid
	}
	agentID := ""
	if claims.AgentID != nil {
		agentID = claims.AgentID.String()
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, tokenClaims{
		OrganizationID: claims.OrganizationID.String(),
		WorkspaceID:    claims.WorkspaceID.String(),
		ActorID:        claims.ActorID.String(),
		AgentID:        agentID,
		RunID:          claims.RunID.String(),
		ResourceIDs:    uuidStrings(claims.ResourceIDs),
		Actions:        append([]string(nil), claims.Actions...),
		Nonce:          claims.Nonce,
		RegisteredClaims: jwt.RegisteredClaims{
			Audience:  jwt.ClaimStrings{s.audience},
			ExpiresAt: jwt.NewNumericDate(claims.ExpiresAt.UTC()),
			IssuedAt:  jwt.NewNumericDate(time.Now().UTC()),
			ID:        claims.Nonce,
		},
	})
	signed, err := token.SignedString(s.secret)
	if err != nil {
		return "", "", domain.ErrCapabilityInvalid
	}
	return signed, service.HashToken(signed), nil
}

func (s *Signer) Verify(raw string, now time.Time) (service.Claims, error) {
	parsed := &tokenClaims{}
	token, err := jwt.ParseWithClaims(raw, parsed, func(token *jwt.Token) (any, error) {
		if token.Method != jwt.SigningMethodHS256 {
			return nil, domain.ErrCapabilityInvalid
		}
		return s.secret, nil
	}, jwt.WithAudience(s.audience), jwt.WithExpirationRequired(), jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}), jwt.WithTimeFunc(func() time.Time { return now }))
	if err != nil || !token.Valid || parsed.Nonce == "" {
		return service.Claims{}, domain.ErrCapabilityInvalid
	}
	claims, err := parsed.domainClaims(s.audience)
	if err != nil {
		return service.Claims{}, err
	}
	return claims, nil
}

func (c tokenClaims) domainClaims(audience string) (service.Claims, error) {
	organizationID, err := uuid.Parse(c.OrganizationID)
	if err != nil {
		return service.Claims{}, domain.ErrCapabilityInvalid
	}
	workspaceID, err := uuid.Parse(c.WorkspaceID)
	if err != nil {
		return service.Claims{}, domain.ErrCapabilityInvalid
	}
	actorID, err := uuid.Parse(c.ActorID)
	if err != nil {
		return service.Claims{}, domain.ErrCapabilityInvalid
	}
	runID, err := uuid.Parse(c.RunID)
	if err != nil {
		return service.Claims{}, domain.ErrCapabilityInvalid
	}
	var agentID *uuid.UUID
	if c.AgentID != "" {
		parsed, parseErr := uuid.Parse(c.AgentID)
		if parseErr != nil {
			return service.Claims{}, domain.ErrCapabilityInvalid
		}
		agentID = &parsed
	}
	resourceIDs := make([]uuid.UUID, 0, len(c.ResourceIDs))
	for _, raw := range c.ResourceIDs {
		id, parseErr := uuid.Parse(raw)
		if parseErr != nil {
			return service.Claims{}, domain.ErrCapabilityInvalid
		}
		resourceIDs = append(resourceIDs, id)
	}
	expiresAt := time.Time{}
	if c.ExpiresAt != nil {
		expiresAt = c.ExpiresAt.Time
	}
	return service.Claims{
		OrganizationID: organizationID, WorkspaceID: workspaceID, ActorID: actorID, AgentID: agentID,
		RunID: runID, ResourceIDs: resourceIDs, Actions: append([]string(nil), c.Actions...),
		Audience: audience, ExpiresAt: expiresAt, Nonce: c.Nonce,
	}, nil
}

func uuidStrings(ids []uuid.UUID) []string {
	out := make([]string, len(ids))
	for i, id := range ids {
		out[i] = id.String()
	}
	return out
}
