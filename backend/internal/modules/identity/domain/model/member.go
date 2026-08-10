package model

import (
	"strings"
	"time"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/identity/domain"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

type MemberStatus string

const (
	MemberStatusActive   MemberStatus = "active"
	MemberStatusDisabled MemberStatus = "disabled"
)

type Member struct {
	ID           kernel.MemberID
	Email        string
	DisplayName  string
	PasswordHash string
	Status       MemberStatus
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

func NewMember(email, displayName, passwordHash string) *Member {
	now := time.Now().UTC()
	return &Member{
		ID:           kernel.NewMemberID(),
		Email:        email,
		DisplayName:  displayName,
		PasswordHash: passwordHash,
		Status:       MemberStatusActive,
		CreatedAt:    now,
		UpdatedAt:    now,
	}
}

func (m *Member) Authenticate(plainPassword string, hasher PasswordHasher) error {
	if m.Status != MemberStatusActive {
		return domain.ErrMemberDisabled
	}
	if !hasher.Compare(m.PasswordHash, plainPassword) {
		return domain.ErrInvalidCredentials
	}
	return nil
}

func (m *Member) Disable() {
	m.Status = MemberStatusDisabled
	m.UpdatedAt = time.Now().UTC()
}

func (m *Member) UpdateDisplayName(displayName string) error {
	name := strings.TrimSpace(displayName)
	if name == "" {
		return domain.ErrInvalidProfile
	}
	m.DisplayName = name
	m.UpdatedAt = time.Now().UTC()
	return nil
}

func (m *Member) ChangePassword(currentPlain, newPlain string, hasher PasswordHasher) error {
	if len(newPlain) < 8 {
		return domain.ErrInvalidProfile
	}
	if !hasher.Compare(m.PasswordHash, currentPlain) {
		return domain.ErrInvalidCredentials
	}
	hash, err := hasher.Hash(newPlain)
	if err != nil {
		return err
	}
	m.PasswordHash = hash
	m.UpdatedAt = time.Now().UTC()
	return nil
}

func (m *Member) ResetPassword(newPlain string, hasher PasswordHasher) error {
	if len(newPlain) < 8 {
		return domain.ErrInvalidProfile
	}
	hash, err := hasher.Hash(newPlain)
	if err != nil {
		return err
	}
	m.PasswordHash = hash
	m.UpdatedAt = time.Now().UTC()
	return nil
}

func (m *Member) SetStatus(status MemberStatus) {
	m.Status = status
	m.UpdatedAt = time.Now().UTC()
}
