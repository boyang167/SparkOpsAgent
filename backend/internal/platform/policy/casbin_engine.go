package policy

import (
	"context"
	_ "embed"
	"fmt"

	"github.com/casbin/casbin/v3"
	"github.com/casbin/casbin/v3/model"
	gormadapter "github.com/casbin/gorm-adapter/v3"
	"gorm.io/gorm"
)

//go:embed model.conf
var modelConf string

type CasbinEngine struct {
	enforcer *casbin.Enforcer
}

func NewCasbinEngine(db *gorm.DB) (*CasbinEngine, error) {
	if err := db.Exec("CREATE SCHEMA IF NOT EXISTS governance").Error; err != nil {
		return nil, fmt.Errorf("create governance schema: %w", err)
	}
	adapter, err := gormadapter.NewAdapterByDBUseTableName(db, "governance", "casbin_rule")
	if err != nil {
		return nil, fmt.Errorf("casbin adapter: %w", err)
	}
	m, err := model.NewModelFromString(modelConf)
	if err != nil {
		return nil, fmt.Errorf("casbin model: %w", err)
	}
	enforcer, err := casbin.NewEnforcer(m, adapter)
	if err != nil {
		return nil, fmt.Errorf("casbin enforcer: %w", err)
	}
	return &CasbinEngine{enforcer: enforcer}, nil
}

func (e *CasbinEngine) Enforce(_ context.Context, subject, domain, object, action string) (bool, error) {
	return e.enforcer.Enforce(subject, domain, object, action)
}

func (e *CasbinEngine) AddPolicy(subject, domain, object, action string) error {
	_, err := e.enforcer.AddPolicy(subject, domain, object, action)
	return err
}

func (e *CasbinEngine) RemovePolicy(subject, domain, object, action string) error {
	_, err := e.enforcer.RemovePolicy(subject, domain, object, action)
	return err
}

func (e *CasbinEngine) RemoveFilteredPolicy(fieldIndex int, fieldValues ...string) error {
	_, err := e.enforcer.RemoveFilteredPolicy(fieldIndex, fieldValues...)
	return err
}

func (e *CasbinEngine) AddGroupingPolicy(subject, role, domain string) error {
	_, err := e.enforcer.AddGroupingPolicy(subject, role, domain)
	return err
}

func (e *CasbinEngine) RemoveGroupingPolicy(subject, role, domain string) error {
	_, err := e.enforcer.RemoveGroupingPolicy(subject, role, domain)
	return err
}

func (e *CasbinEngine) RemoveFilteredGroupingPolicy(fieldIndex int, fieldValues ...string) error {
	_, err := e.enforcer.RemoveFilteredGroupingPolicy(fieldIndex, fieldValues...)
	return err
}

// SyncRoleAPIPolicies replaces Casbin p-policies for a role's HTTP API bindings.
// Each policy is [path, method].
func (e *CasbinEngine) SyncRoleAPIPolicies(roleID, workspaceID string, policies [][2]string) error {
	if err := e.RemoveFilteredPolicy(0, roleID, workspaceID); err != nil {
		return err
	}
	for _, p := range policies {
		if p[0] == "" || p[1] == "" {
			continue
		}
		if err := e.AddPolicy(roleID, workspaceID, p[0], p[1]); err != nil {
			return err
		}
	}
	return e.SavePolicy()
}

func (e *CasbinEngine) SavePolicy() error {
	return e.enforcer.SavePolicy()
}

// SyncProjectMemberRole replaces Casbin policies for a project member.
func (e *CasbinEngine) SyncProjectMemberRole(memberID, workspaceID, projectID, role string) error {
	obj := ProjectObject(projectID)
	if err := e.RemoveFilteredPolicy(0, memberID, workspaceID, obj); err != nil {
		return err
	}
	for _, act := range actionsForProjectRole(role) {
		if err := e.AddPolicy(memberID, workspaceID, obj, act); err != nil {
			return err
		}
	}
	return e.SavePolicy()
}

func (e *CasbinEngine) RemoveProjectMember(memberID, workspaceID, projectID string) error {
	obj := ProjectObject(projectID)
	if err := e.RemoveFilteredPolicy(0, memberID, workspaceID, obj); err != nil {
		return err
	}
	return e.SavePolicy()
}

func actionsForProjectRole(role string) []string {
	switch role {
	case "lead":
		return []string{ActionRead, ActionWrite, ActionAdmin}
	case "member":
		return []string{ActionRead, ActionWrite}
	default:
		return []string{ActionRead}
	}
}

var _ Engine = (*CasbinEngine)(nil)
