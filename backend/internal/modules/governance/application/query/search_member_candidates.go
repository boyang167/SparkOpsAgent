package query

import (
	"context"
	"strings"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/governance/infrastructure/acl"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

const (
	MinMemberSearchQueryLen = 2
	MaxMemberSearchLimit    = 20
	DefaultMemberSearchLimit = 10
)

type MemberCandidateView struct {
	MemberID    string `json:"member_id"`
	Email       string `json:"email"`
	DisplayName string `json:"display_name"`
}

type SearchMemberCandidatesHandler struct {
	identity *acl.IdentityACL
}

func NewSearchMemberCandidatesHandler(identity *acl.IdentityACL) *SearchMemberCandidatesHandler {
	return &SearchMemberCandidatesHandler{identity: identity}
}

func (h *SearchMemberCandidatesHandler) Handle(
	ctx context.Context,
	wsID kernel.WorkspaceID,
	query string,
	limit int,
) ([]MemberCandidateView, error) {
	query = strings.TrimSpace(strings.ToLower(query))
	if len(query) < MinMemberSearchQueryLen {
		return []MemberCandidateView{}, nil
	}
	if limit <= 0 {
		limit = DefaultMemberSearchLimit
	}
	if limit > MaxMemberSearchLimit {
		limit = MaxMemberSearchLimit
	}

	rows, err := h.identity.SearchInviteCandidates(ctx, wsID, query, limit)
	if err != nil {
		return nil, err
	}
	out := make([]MemberCandidateView, 0, len(rows))
	for _, row := range rows {
		out = append(out, MemberCandidateView{
			MemberID:    row.MemberID.String(),
			Email:       row.Email,
			DisplayName: row.DisplayName,
		})
	}
	return out, nil
}
