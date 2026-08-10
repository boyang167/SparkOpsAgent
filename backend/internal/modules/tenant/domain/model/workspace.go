package model

import (
	"strings"
	"time"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/tenant/domain"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

type OrgQuota struct {
	MaxWorkspaces int
	MaxAgents     int
	MaxProjects   int
	TokenBudget   int64
}

type Organization struct {
	ID        kernel.OrgID
	Name      string
	Slug      string
	Status    string
	Quota     OrgQuota
	CreatedAt time.Time
}

func DefaultOrgQuota() OrgQuota {
	return OrgQuota{MaxWorkspaces: 50, MaxAgents: 500, MaxProjects: 1000, TokenBudget: 0}
}

func NewOrganization(name, slug string) *Organization {
	return &Organization{
		ID:        kernel.NewOrgID(),
		Name:      strings.TrimSpace(name),
		Slug:      normalizeSlug(slug),
		Status:    "active",
		Quota:     DefaultOrgQuota(),
		CreatedAt: time.Now().UTC(),
	}
}

func (q OrgQuota) CanCreateWorkspace(current int) error {
	if q.MaxWorkspaces > 0 && current >= q.MaxWorkspaces {
		return domain.ErrQuotaExceeded
	}
	return nil
}

func (q OrgQuota) CanCreateAgent(current int) error {
	if q.MaxAgents > 0 && current >= q.MaxAgents {
		return domain.ErrQuotaExceeded
	}
	return nil
}

func (q OrgQuota) CanCreateProject(current int) error {
	if q.MaxProjects > 0 && current >= q.MaxProjects {
		return domain.ErrQuotaExceeded
	}
	return nil
}

type WorkspaceQuota struct {
	MaxAgents   int
	MaxProjects int
}

type Workspace struct {
	ID        kernel.WorkspaceID
	OrgID     kernel.OrgID
	Name      string
	Slug      string
	Status    string
	Quota     WorkspaceQuota
	CreatedAt time.Time
}

func NewWorkspace(orgID kernel.OrgID, name, slug string, quota WorkspaceQuota) *Workspace {
	if quota.MaxAgents <= 0 {
		quota.MaxAgents = 10
	}
	if quota.MaxProjects <= 0 {
		quota.MaxProjects = 20
	}
	return &Workspace{
		ID:        kernel.NewWorkspaceID(),
		OrgID:     orgID,
		Name:      strings.TrimSpace(name),
		Slug:      normalizeSlug(slug),
		Status:    "active",
		Quota:     quota,
		CreatedAt: time.Now().UTC(),
	}
}

func (w *Workspace) CanCreateAgent(current int) error {
	if current >= w.Quota.MaxAgents {
		return domain.ErrQuotaExceeded
	}
	return nil
}

func normalizeSlug(slug string) string {
	return strings.ToLower(strings.TrimSpace(slug))
}
