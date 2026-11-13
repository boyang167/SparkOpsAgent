package filesystemgateway

import (
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

type capabilityOptions struct {
	sessionID string
	fileID    string
	action    string
	expires   time.Time
}

func capabilityToken(t *testing.T, opts capabilityOptions) string {
	t.Helper()
	if opts.expires.IsZero() {
		opts.expires = time.Now().Add(10 * time.Minute)
	}
	nonce := uuid.NewString()
	claims := CapabilityClaims{
		Action: opts.action,
		Nonce:  nonce,
		RegisteredClaims: jwt.RegisteredClaims{
			Audience:  jwt.ClaimStrings{testCapabilityAudience},
			ExpiresAt: jwt.NewNumericDate(opts.expires),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}
	if opts.sessionID != "" {
		claims.SessionID = opts.sessionID
	}
	if opts.fileID != "" {
		claims.FileID = opts.fileID
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, &claims)
	signed, err := token.SignedString([]byte(testCapabilitySecret))
	if err != nil {
		t.Fatalf("sign capability token: %v", err)
	}
	return signed
}
