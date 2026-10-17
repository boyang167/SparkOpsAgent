package outbound

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/sparkops/sparkops-agent/backend/internal/platform/messaging"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

const OperationTypeMemberRoleSync = "governance.casbin.member-role.sync.v1"

type MemberRoleSyncPayload struct {
	EventID     string `json:"event_id"`
	WorkspaceID string `json:"workspace_id"`
	RoleID      string `json:"role_id"`
	MemberID    string `json:"member_id"`
	Add         bool   `json:"add"`
}

type MemberRoleSyncer interface {
	SyncMemberRole(context.Context, kernel.WorkspaceID, kernel.RoleID, kernel.MemberID, bool) error
}

func NewMemberRoleSyncHandler(syncer MemberRoleSyncer) messaging.OperationHandler {
	return func(ctx context.Context, operation messaging.Operation) error {
		var payload MemberRoleSyncPayload
		if err := json.Unmarshal(operation.Payload, &payload); err != nil {
			return messaging.Permanent(fmt.Errorf("decode member role sync operation: %w", err))
		}
		workspaceID, err := kernel.ParseWorkspaceID(payload.WorkspaceID)
		if err != nil {
			return messaging.Permanent(err)
		}
		roleID, err := kernel.ParseRoleID(payload.RoleID)
		if err != nil {
			return messaging.Permanent(err)
		}
		memberID, err := kernel.ParseMemberID(payload.MemberID)
		if err != nil {
			return messaging.Permanent(err)
		}
		return syncer.SyncMemberRole(ctx, workspaceID, roleID, memberID, payload.Add)
	}
}
