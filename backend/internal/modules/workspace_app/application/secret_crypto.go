package application

import (
	"encoding/json"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/workspace_app/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/crypto"
)

const maskedValue = "******"

type SecretCrypto struct {
	box *crypto.SecretBox
}

func NewSecretCrypto(box *crypto.SecretBox) *SecretCrypto {
	return &SecretCrypto{box: box}
}

func (c *SecretCrypto) EncryptCredentials(creds model.SecretCredentials) (string, error) {
	raw, err := model.MarshalCredentials(creds)
	if err != nil {
		return "", err
	}
	return c.box.Encrypt(raw)
}

func (c *SecretCrypto) DecryptCredentials(blob string) (model.SecretCredentials, error) {
	var creds model.SecretCredentials
	if blob == "" {
		return creds, nil
	}
	plain, err := c.box.Decrypt(blob)
	if err != nil {
		return creds, err
	}
	if plain == "" {
		return creds, nil
	}
	err = json.Unmarshal([]byte(plain), &creds)
	return creds, err
}

func CredentialFlags(creds model.SecretCredentials) map[string]bool {
	return map[string]bool{
		"has_password":    creds.Password != "",
		"has_token":       creds.Token != "",
		"has_api_key":     creds.APIKey != "",
		"has_private_key": creds.PrivateKey != "",
		"has_public_key":  creds.PublicKey != "",
		"has_certificate": creds.Certificate != "",
		"has_extra_config": creds.ExtraConfig != "",
	}
}

func MaskedCredentials(creds model.SecretCredentials) map[string]string {
	mask := func(v string) string {
		if v == "" {
			return ""
		}
		return maskedValue
	}
	return map[string]string{
		"password":     mask(creds.Password),
		"token":        mask(creds.Token),
		"api_key":      mask(creds.APIKey),
		"private_key":  mask(creds.PrivateKey),
		"public_key":   mask(creds.PublicKey),
		"certificate":  mask(creds.Certificate),
		"extra_config": mask(creds.ExtraConfig),
	}
}
