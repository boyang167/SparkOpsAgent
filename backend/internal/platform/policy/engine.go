package policy

import (
	"context"
)

// Engine abstracts Casbin/OPA-style policy enforcement.
type Engine interface {
	Enforce(ctx context.Context, subject, domain, object, action string) (bool, error)
	AddPolicy(subject, domain, object, action string) error
	RemovePolicy(subject, domain, object, action string) error
	RemoveFilteredPolicy(fieldIndex int, fieldValues ...string) error
	AddGroupingPolicy(subject, role, domain string) error
	RemoveGroupingPolicy(subject, role, domain string) error
	RemoveFilteredGroupingPolicy(fieldIndex int, fieldValues ...string) error
	SyncRoleAPIPolicies(roleID, workspaceID string, policies [][2]string) error
	SavePolicy() error
}

const (
	ObjectProjectPrefix = "project:"
	ActionRead          = "read"
	ActionWrite         = "write"
	ActionAdmin         = "admin"
)

func ProjectObject(projectID string) string {
	return ObjectProjectPrefix + projectID
}
