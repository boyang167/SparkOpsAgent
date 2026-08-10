package authn

import (
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"

	"github.com/sparkops/sparkops-agent/backend/internal/platform/config"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

const claimMemberID = "member_id"
const claimWorkspaceID = "workspace_id"
const claimRoleID = "role_id"
const claimTokenType = "token_type"

type TokenProvider struct {
	secret     []byte
	expireHour int
}

func NewTokenProvider(cfg config.JWTConfig) *TokenProvider {
	secret := cfg.Secret
	if secret == "" {
		secret = "dev-only-change-me"
	}
	expire := cfg.ExpireHour
	if expire <= 0 {
		expire = 24
	}
	return &TokenProvider{secret: []byte(secret), expireHour: expire}
}

func (p *TokenProvider) Issue(memberID kernel.MemberID) (string, error) {
	return p.IssueScoped(memberID, kernel.WorkspaceID{}, nil, p.expireHour)
}

func (p *TokenProvider) IssueScoped(
	memberID kernel.MemberID,
	workspaceID kernel.WorkspaceID,
	roleID *kernel.RoleID,
	expireHours int,
) (string, error) {
	if expireHours <= 0 {
		expireHours = p.expireHour
	}
	claims := jwt.MapClaims{
		claimMemberID: memberID.String(),
		"exp":         time.Now().Add(time.Duration(expireHours) * time.Hour).Unix(),
		"iat":         time.Now().Unix(),
	}
	if workspaceID.UUID != uuid.Nil {
		claims[claimWorkspaceID] = workspaceID.String()
		claims[claimTokenType] = "api_token"
	}
	if roleID != nil {
		claims[claimRoleID] = roleID.String()
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(p.secret)
}

func (p *TokenProvider) ParseMemberID(tokenString string) (kernel.MemberID, error) {
	claims, err := p.parseClaims(tokenString)
	if err != nil {
		return kernel.MemberID{}, err
	}
	raw, ok := claims[claimMemberID].(string)
	if !ok || raw == "" {
		return kernel.MemberID{}, errors.New("missing member_id")
	}
	return kernel.ParseMemberID(raw)
}

func (p *TokenProvider) parseClaims(tokenString string) (jwt.MapClaims, error) {
	token, err := jwt.Parse(tokenString, func(t *jwt.Token) (interface{}, error) {
		if t.Method != jwt.SigningMethodHS256 {
			return nil, errors.New("unexpected signing method")
		}
		return p.secret, nil
	})
	if err != nil || !token.Valid {
		return nil, errors.New("invalid token")
	}
	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return nil, errors.New("invalid claims")
	}
	return claims, nil
}
