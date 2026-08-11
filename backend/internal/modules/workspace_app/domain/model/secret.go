package model

import (
	"encoding/json"
	"strings"
	"time"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/workspace_app/domain"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

type SecretStatus string

const (
	SecretStatusActive   SecretStatus = "active"
	SecretStatusInactive SecretStatus = "inactive"
)

type SecretCredentials struct {
	Password    string `json:"password,omitempty"`
	Token       string `json:"token,omitempty"`
	APIKey      string `json:"api_key,omitempty"`
	PrivateKey  string `json:"private_key,omitempty"`
	PublicKey   string `json:"public_key,omitempty"`
	Certificate string `json:"certificate,omitempty"`
	ExtraConfig string `json:"extra_config,omitempty"`
}

type SecretName struct{ value string }

func NewSecretName(raw string) (SecretName, error) {
	v := strings.TrimSpace(raw)
	if v == "" || len(v) > 255 {
		return SecretName{}, domain.ErrInvalidSecretName
	}
	return SecretName{value: v}, nil
}

func (n SecretName) String() string { return n.value }

type Secret struct {
	ID              kernel.SecretID
	WorkspaceID     kernel.WorkspaceID
	Name            SecretName
	DisplayName     string
	Description     string
	ServiceType     string
	ServiceURL      string
	Username        string
	CredentialBlob  string
	Status          SecretStatus
	IsDefault       bool
	IsShared        bool
	CreatedBy       kernel.MemberID
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

func NewSecret(
	wsID kernel.WorkspaceID,
	name SecretName,
	displayName, description, serviceType, serviceURL, username string,
	credentialBlob string,
	status SecretStatus,
	isDefault, isShared bool,
	createdBy kernel.MemberID,
) *Secret {
	now := time.Now().UTC()
	if status == "" {
		status = SecretStatusActive
	}
	return &Secret{
		ID: kernel.NewSecretID(), WorkspaceID: wsID, Name: name,
		DisplayName: strings.TrimSpace(displayName), Description: description,
		ServiceType: strings.TrimSpace(serviceType), ServiceURL: strings.TrimSpace(serviceURL),
		Username: strings.TrimSpace(username), CredentialBlob: credentialBlob,
		Status: status, IsDefault: isDefault, IsShared: isShared,
		CreatedBy: createdBy, CreatedAt: now, UpdatedAt: now,
	}
}

func (s *Secret) Update(
	displayName, description, serviceType, serviceURL, username, credentialBlob string,
	status SecretStatus,
	isDefault, isShared bool,
) {
	s.DisplayName = strings.TrimSpace(displayName)
	s.Description = description
	s.ServiceType = strings.TrimSpace(serviceType)
	s.ServiceURL = strings.TrimSpace(serviceURL)
	s.Username = strings.TrimSpace(username)
	if credentialBlob != "" {
		s.CredentialBlob = credentialBlob
	}
	if status != "" {
		s.Status = status
	}
	s.IsDefault = isDefault
	s.IsShared = isShared
	s.UpdatedAt = time.Now().UTC()
}

func (s *Secret) CredentialsJSON() (SecretCredentials, error) {
	var creds SecretCredentials
	if s.CredentialBlob == "" {
		return creds, nil
	}
	err := json.Unmarshal([]byte(s.CredentialBlob), &creds)
	return creds, err
}

func MarshalCredentials(creds SecretCredentials) (string, error) {
	b, err := json.Marshal(creds)
	if err != nil {
		return "", err
	}
	return string(b), nil
}
