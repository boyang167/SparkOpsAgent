package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"io"

	"github.com/sparkops/sparkops-agent/backend/internal/platform/config"
)

type SecretBox struct {
	gcm cipher.AEAD
}

func NewSecretBoxFromConfig(cfg *config.Config) (*SecretBox, error) {
	keyMaterial := ""
	if cfg != nil {
		keyMaterial = cfg.Security.SecretEncryptionKey
		if keyMaterial == "" {
			keyMaterial = cfg.JWT.Secret
		}
	}
	if keyMaterial == "" {
		return nil, errors.New("secret encryption key is required")
	}
	sum := sha256.Sum256([]byte(keyMaterial))
	return NewSecretBox(sum[:])
}

func NewSecretBox(key []byte) (*SecretBox, error) {
	if len(key) != 32 {
		return nil, fmt.Errorf("secret box key must be 32 bytes, got %d", len(key))
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	return &SecretBox{gcm: gcm}, nil
}

func (b *SecretBox) Encrypt(plaintext string) (string, error) {
	if plaintext == "" {
		return "", nil
	}
	nonce := make([]byte, b.gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}
	ciphertext := b.gcm.Seal(nonce, nonce, []byte(plaintext), nil)
	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

func (b *SecretBox) Decrypt(encoded string) (string, error) {
	if encoded == "" {
		return "", nil
	}
	raw, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return "", err
	}
	nonceSize := b.gcm.NonceSize()
	if len(raw) < nonceSize {
		return "", errors.New("invalid secret ciphertext")
	}
	nonce, ciphertext := raw[:nonceSize], raw[nonceSize:]
	plain, err := b.gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", err
	}
	return string(plain), nil
}
