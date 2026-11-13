package service

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"time"

	"github.com/google/uuid"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/domain"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/domain/model"
)

const AgentCapabilityAudience = "sparkops-agent-capability"

type DecisionResult string

const (
	DecisionAllow DecisionResult = "ALLOW"
	DecisionDeny  DecisionResult = "DENY"
)

const (
	SourceGrant       = "grant"
	SourceOwner       = "owner"
	SourceAdminBypass = "admin_bypass"
)

type Decision struct {
	Result        DecisionResult
	ReasonCode    string
	PolicyID      string
	PolicyVersion string
	TraceID       string
}

type Subject struct {
	ActorID        uuid.UUID
	OrganizationID uuid.UUID
	WorkspaceID    uuid.UUID
	Authenticated  bool
	Entitled       bool
	RBACAllowed    bool
	TraceID        string
}

type Resource struct {
	ID             uuid.UUID
	OrganizationID uuid.UUID
	WorkspaceID    uuid.UUID
	ParentID       *uuid.UUID
}

type Relation struct {
	ResourceID    uuid.UUID
	Effect        model.GrantEffect
	Inherited     bool
	Permission    model.ResourcePermission
	Source        string
	PolicyID      string
	PolicyVersion string
	ABACDeny      bool
}

func Decide(subject Subject, action model.ResourcePermission, resource Resource, relations ...Relation) Decision {
	switch {
	case !subject.Authenticated:
		return denied(subject, "authentication", "", "")
	case subject.OrganizationID != resource.OrganizationID || subject.WorkspaceID != resource.WorkspaceID:
		return denied(subject, "organization_boundary", "", "")
	case !subject.Entitled:
		return denied(subject, "entitlement", "", "")
	case !subject.RBACAllowed:
		return denied(subject, "rbac", "", "")
	}

	var (
		adminDeny, explicitDeny, abacDeny, owner, grantAllow, adminBypass         Relation
		hasAdminDeny, hasExplicitDeny, hasABACDeny, hasOwner, hasGrant, hasBypass bool
	)
	for _, relation := range relations {
		if relation.ResourceID != resource.ID && !relation.Inherited {
			continue
		}
		if relation.Effect == model.GrantEffectDeny && covers(relation.Permission, action) {
			if relation.Permission == model.ResourcePermissionAdmin && !hasAdminDeny {
				adminDeny, hasAdminDeny = relation, true
			} else if relation.Permission != model.ResourcePermissionAdmin && !hasExplicitDeny {
				explicitDeny, hasExplicitDeny = relation, true
			}
		}
		if relation.ABACDeny && !hasABACDeny {
			abacDeny, hasABACDeny = relation, true
		}
		if relation.Effect != model.GrantEffectAllow || !covers(relation.Permission, action) || relation.ABACDeny {
			continue
		}
		switch relation.Source {
		case SourceOwner:
			if !hasOwner {
				owner, hasOwner = relation, true
			}
		case SourceAdminBypass:
			if !hasBypass {
				adminBypass, hasBypass = relation, true
			}
		default:
			if !hasGrant {
				grantAllow, hasGrant = relation, true
			}
		}
	}
	switch {
	case hasAdminDeny:
		return denied(subject, "admin_deny", adminDeny.PolicyID, adminDeny.PolicyVersion)
	case hasExplicitDeny:
		return denied(subject, "explicit_deny", explicitDeny.PolicyID, explicitDeny.PolicyVersion)
	case hasABACDeny:
		return denied(subject, "abac_deny", abacDeny.PolicyID, abacDeny.PolicyVersion)
	case hasOwner:
		return allowed(subject, "owner", owner.PolicyID, owner.PolicyVersion)
	case hasGrant:
		return allowed(subject, "grant_allow", grantAllow.PolicyID, grantAllow.PolicyVersion)
	case hasBypass:
		return allowed(subject, "admin_bypass", adminBypass.PolicyID, adminBypass.PolicyVersion)
	default:
		return denied(subject, "default_deny", "", "")
	}
}

func covers(granted, requested model.ResourcePermission) bool {
	return granted == model.ResourcePermissionAdmin || granted == requested
}

func allowed(subject Subject, reason, policyID, policyVersion string) Decision {
	return Decision{Result: DecisionAllow, ReasonCode: reason, PolicyID: policyID, PolicyVersion: policyVersion, TraceID: subject.TraceID}
}

func denied(subject Subject, reason, policyID, policyVersion string) Decision {
	return Decision{Result: DecisionDeny, ReasonCode: reason, PolicyID: policyID, PolicyVersion: policyVersion, TraceID: subject.TraceID}
}

type Claims struct {
	OrganizationID uuid.UUID
	WorkspaceID    uuid.UUID
	ActorID        uuid.UUID
	AgentID        *uuid.UUID
	RunID          uuid.UUID
	ResourceIDs    []uuid.UUID
	Actions        []string
	Audience       string
	ExpiresAt      time.Time
	Nonce          string
}

type CapabilityRequest struct {
	OrganizationID       uuid.UUID
	WorkspaceID          uuid.UUID
	ActorID              uuid.UUID
	AgentID              *uuid.UUID
	RunID                uuid.UUID
	ResourceIDs          []uuid.UUID
	DelegatorResourceIDs []uuid.UUID
	Actions              []string
	Audience             string
	ExpiresAt            time.Time
}

type RunScope struct {
	ResourceIDs []uuid.UUID
	Actions     []string
}

type CapabilityToken struct {
	ResourceIDs []uuid.UUID
	Claims      Claims
	Token       string
	Hash        string
}

type TokenSigner interface {
	Sign(Claims) (string, string, error)
}

type CapabilityIssuer struct {
	signer TokenSigner
}

func NewCapabilityIssuer(signer TokenSigner) *CapabilityIssuer {
	return &CapabilityIssuer{signer: signer}
}

func (i *CapabilityIssuer) Issue(ctx context.Context, request CapabilityRequest, scope RunScope) (CapabilityToken, error) {
	if err := ctx.Err(); err != nil {
		return CapabilityToken{}, err
	}
	resources := intersectUUIDs(request.ResourceIDs, scope.ResourceIDs)
	if request.DelegatorResourceIDs != nil {
		resources = intersectUUIDs(resources, request.DelegatorResourceIDs)
	}
	actions := append([]string(nil), request.Actions...)
	if len(scope.Actions) > 0 {
		actions = intersectStrings(request.Actions, scope.Actions)
	}
	if len(resources) == 0 || (len(request.Actions) > 0 && len(scope.Actions) > 0 && len(actions) == 0) {
		return CapabilityToken{}, domain.ErrCapabilityScopeEmpty
	}
	nonce, err := randomNonce()
	if err != nil {
		return CapabilityToken{}, err
	}
	claims := Claims{
		OrganizationID: request.OrganizationID,
		WorkspaceID:    request.WorkspaceID,
		ActorID:        request.ActorID,
		AgentID:        request.AgentID,
		RunID:          request.RunID,
		ResourceIDs:    resources,
		Actions:        actions,
		Audience:       request.Audience,
		ExpiresAt:      request.ExpiresAt.UTC(),
		Nonce:          nonce,
	}
	token := CapabilityToken{ResourceIDs: append([]uuid.UUID(nil), resources...), Claims: claims}
	if i.signer == nil {
		return token, nil
	}
	signed, hash, err := i.signer.Sign(claims)
	if err != nil {
		return CapabilityToken{}, err
	}
	token.Token = signed
	token.Hash = hash
	return token, nil
}

func HashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

func intersectUUIDs(left, right []uuid.UUID) []uuid.UUID {
	allowed := make(map[uuid.UUID]struct{}, len(right))
	for _, id := range right {
		if id != uuid.Nil {
			allowed[id] = struct{}{}
		}
	}
	seen := make(map[uuid.UUID]struct{}, len(left))
	out := make([]uuid.UUID, 0, len(left))
	for _, id := range left {
		if _, ok := allowed[id]; !ok {
			continue
		}
		if _, dup := seen[id]; dup {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	return out
}

func intersectStrings(left, right []string) []string {
	allowed := make(map[string]struct{}, len(right))
	for _, value := range right {
		allowed[value] = struct{}{}
	}
	seen := map[string]struct{}{}
	out := make([]string, 0, len(left))
	for _, value := range left {
		if _, ok := allowed[value]; !ok {
			continue
		}
		if _, dup := seen[value]; dup {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	return out
}

func randomNonce() (string, error) {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}
