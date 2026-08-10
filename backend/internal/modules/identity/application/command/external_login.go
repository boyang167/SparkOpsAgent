package command

import (
	"context"
	"strings"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/identity/domain"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/identity/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/identity/domain/repository"
	idauth "github.com/sparkops/sparkops-agent/backend/internal/modules/identity/infrastructure/auth"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/authn"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

type ExternalLoginResult struct {
	Token    string
	MemberID kernel.MemberID
	Email    string
}

type ResolveExternalIdentityHandler struct {
	members    repository.MemberRepository
	external   repository.ExternalIdentityRepository
	hasher     model.PasswordHasher
	tokens     *authn.TokenProvider
}

func NewResolveExternalIdentityHandler(
	members repository.MemberRepository,
	external repository.ExternalIdentityRepository,
	tokens *authn.TokenProvider,
) *ResolveExternalIdentityHandler {
	return &ResolveExternalIdentityHandler{
		members: members, external: external, hasher: model.BcryptHasher{}, tokens: tokens,
	}
}

func (h *ResolveExternalIdentityHandler) Handle(ctx context.Context, provider, subject, email, displayName string) (*ExternalLoginResult, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	link, err := h.external.FindByProviderSubject(ctx, provider, subject)
	if err != nil {
		return nil, err
	}
	var member *model.Member
	if link != nil {
		member, err = h.members.FindByID(ctx, link.MemberID)
		if err != nil {
			return nil, err
		}
	} else {
		member, err = h.members.FindByEmail(ctx, email)
		if err != nil {
			if de, ok := kernel.IsDomainError(err); ok && de.Code() == domain.ErrMemberNotFound.Code() {
				hash, hashErr := h.hasher.Hash(idauth.RandomPassword())
				if hashErr != nil {
					return nil, hashErr
				}
				member = model.NewMember(email, displayName, hash)
				if saveErr := h.members.Save(ctx, member); saveErr != nil {
					return nil, saveErr
				}
			} else {
				return nil, err
			}
		}
		if err := h.external.Save(ctx, &repository.ExternalIdentity{
			Provider: provider, Subject: subject, MemberID: member.ID, Email: email,
		}); err != nil {
			return nil, err
		}
	}
	token, err := h.tokens.Issue(member.ID)
	if err != nil {
		return nil, err
	}
	return &ExternalLoginResult{Token: token, MemberID: member.ID, Email: member.Email}, nil
}

type LDAPLoginHandler struct {
	ldap     *idauth.LDAPAuthenticator
	resolver *ResolveExternalIdentityHandler
}

func NewLDAPLoginHandler(ldap *idauth.LDAPAuthenticator, resolver *ResolveExternalIdentityHandler) *LDAPLoginHandler {
	return &LDAPLoginHandler{ldap: ldap, resolver: resolver}
}

func (h *LDAPLoginHandler) Handle(ctx context.Context, username, password string) (*ExternalLoginResult, error) {
	user, err := h.ldap.Authenticate(username, password)
	if err != nil {
		return nil, err
	}
	return h.resolver.Handle(ctx, "ldap", user.Subject, user.Email, user.DisplayName)
}
