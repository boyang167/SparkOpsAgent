package query

import (
	"context"
	"strings"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/tenant/infrastructure/acl"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

const (
	MinOrgMemberSearchQueryLen = 2
	MaxOrgMemberSearchLimit    = 20
	DefaultOrgMemberSearchLimit = 10
)

type OrgMemberCandidateView struct {
	MemberID    string `json:"member_id"`
	Email       string `json:"email"`
	DisplayName string `json:"display_name"`
}

type SearchOrgMemberCandidatesHandler struct {
	identity *acl.IdentityACL
}

func NewSearchOrgMemberCandidatesHandler(identity *acl.IdentityACL) *SearchOrgMemberCandidatesHandler {
	return &SearchOrgMemberCandidatesHandler{identity: identity}
}

func (h *SearchOrgMemberCandidatesHandler) Handle(
	ctx context.Context,
	orgID kernel.OrgID,
	query string,
	limit int,
) ([]OrgMemberCandidateView, error) {
	query = strings.TrimSpace(strings.ToLower(query))
	if len(query) < MinOrgMemberSearchQueryLen {
		return []OrgMemberCandidateView{}, nil
	}
	if limit <= 0 {
		limit = DefaultOrgMemberSearchLimit
	}
	if limit > MaxOrgMemberSearchLimit {
		limit = MaxOrgMemberSearchLimit
	}

	rows, err := h.identity.SearchInviteCandidates(ctx, orgID, query, limit)
	if err != nil {
		return nil, err
	}
	out := make([]OrgMemberCandidateView, 0, len(rows))
	for _, row := range rows {
		out = append(out, OrgMemberCandidateView{
			MemberID:    row.MemberID.String(),
			Email:       row.Email,
			DisplayName: row.DisplayName,
		})
	}
	return out, nil
}
