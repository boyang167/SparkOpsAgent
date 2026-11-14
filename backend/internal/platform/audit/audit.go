package audit

import (
	"context"
	"encoding/json"
	"time"

	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

const (
	ActionOrgCreate           = "org.create"
	ActionOrgStatusUpdate     = "org.status.update"
	ActionOrgMemberInvite     = "org.member.invite"
	ActionOrgMemberRoleUpdate = "org.member.role_update"
	ActionOrgMemberRemove     = "org.member.remove"
	ActionOrgOwnerTransfer    = "org.owner.transfer"
	ActionPlatformOrgCreate   = "platform.org.create"
	ActionPlatformOrgDelete   = "platform.org.delete"
)

type Entry struct {
	ActorID      kernel.MemberID
	Action       string
	ResourceType string
	ResourceID   string
	OrgID        *kernel.OrgID
	Detail       map[string]any
}

type LogView struct {
	ID           string         `json:"id"`
	ActorID      string         `json:"actor_id"`
	Action       string         `json:"action"`
	ResourceType string         `json:"resource_type"`
	ResourceID   string         `json:"resource_id"`
	OrgID        string         `json:"org_id,omitempty"`
	Detail       map[string]any `json:"detail"`
	CreatedAt    string         `json:"created_at"`
}

type Logger interface {
	Log(ctx context.Context, entry Entry) error
}

type Store interface {
	Append(ctx context.Context, entry Entry) error
	List(ctx context.Context, filter ListFilter) ([]LogView, error)
}

type ListFilter struct {
	OrgID                      *kernel.OrgID
	Limit                      int
	Offset                     int
	ActionPrefix               string
	Action                     string
	ActorID                    string
	ResourceID                 string
	ResourceType               string
	WorkspaceID                string
	Decision                   string
	AllowMissingDecisionAction string
	CreatedAtAfter             time.Time
	CreatedAtBefore            time.Time
}

func EncodeDetail(detail map[string]any) json.RawMessage {
	if detail == nil {
		return json.RawMessage(`{}`)
	}
	b, err := json.Marshal(detail)
	if err != nil {
		return json.RawMessage(`{}`)
	}
	return b
}

func DecodeDetail(raw json.RawMessage) map[string]any {
	if len(raw) == 0 {
		return map[string]any{}
	}
	var out map[string]any
	if err := json.Unmarshal(raw, &out); err != nil {
		return map[string]any{}
	}
	return out
}

func FormatTime(t time.Time) string {
	return t.UTC().Format(time.RFC3339)
}
