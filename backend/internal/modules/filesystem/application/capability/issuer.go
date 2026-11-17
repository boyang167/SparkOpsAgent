package capability

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

const (
	uploadAction   = "transfer:write"
	downloadAction = "transfer:read"
)

type Issuer struct {
	secret   []byte
	audience string
}

func NewIssuer(secret, audience string) (*Issuer, error) {
	secret = strings.TrimSpace(secret)
	audience = strings.TrimSpace(audience)
	if secret == "" || audience == "" {
		return nil, errors.New("capability issuer requires secret and audience")
	}
	return &Issuer{secret: []byte(secret), audience: audience}, nil
}

type uploadClaims struct {
	SessionID string `json:"session_id"`
	Action    string `json:"action"`
	Nonce     string `json:"nonce"`
	jwt.RegisteredClaims
}

type downloadClaims struct {
	FileID string `json:"file_id"`
	Action string `json:"action"`
	Nonce  string `json:"nonce"`
	jwt.RegisteredClaims
}

func (i *Issuer) DownloadToken(fileID uuid.UUID, expiresAt time.Time) (string, error) {
	if fileID == uuid.Nil || !expiresAt.After(time.Now()) {
		return "", errors.New("invalid capability scope")
	}
	nonce, err := randomNonce()
	if err != nil {
		return "", err
	}
	claims := downloadClaims{
		FileID: fileID.String(),
		Action: downloadAction,
		Nonce:  nonce,
		RegisteredClaims: jwt.RegisteredClaims{
			Audience: jwt.ClaimStrings{i.audience},
			ExpiresAt: jwt.NewNumericDate(expiresAt.UTC()),
			IssuedAt:  jwt.NewNumericDate(time.Now().UTC()),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString(i.secret)
	if err != nil {
		return "", fmt.Errorf("sign capability token: %w", err)
	}
	return signed, nil
}

func (i *Issuer) UploadToken(sessionID uuid.UUID, expiresAt time.Time) (string, error) {
	if sessionID == uuid.Nil || !expiresAt.After(time.Now()) {
		return "", errors.New("invalid capability scope")
	}
	nonce, err := randomNonce()
	if err != nil {
		return "", err
	}
	claims := uploadClaims{
		SessionID: sessionID.String(),
		Action:    uploadAction,
		Nonce:     nonce,
		RegisteredClaims: jwt.RegisteredClaims{
			Audience: jwt.ClaimStrings{i.audience},
			ExpiresAt: jwt.NewNumericDate(expiresAt.UTC()),
			IssuedAt:  jwt.NewNumericDate(time.Now().UTC()),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString(i.secret)
	if err != nil {
		return "", fmt.Errorf("sign capability token: %w", err)
	}
	return signed, nil
}

func randomNonce() (string, error) {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}
