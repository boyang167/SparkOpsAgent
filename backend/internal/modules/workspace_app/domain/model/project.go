package model

import (
	"strings"
	"time"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/workspace_app/domain"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/workspace_app/domain/event"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/events"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

type ProjectStatus string

const (
	ProjectStatusActive   ProjectStatus = "active"
	ProjectStatusArchived ProjectStatus = "archived"
)

type ProjectName struct {
	value string
}

func NewProjectName(raw string) (ProjectName, error) {
	v := strings.TrimSpace(raw)
	if v == "" || len(v) > 255 {
		return ProjectName{}, domain.ErrInvalidProjectName
	}
	return ProjectName{value: v}, nil
}

func (n ProjectName) String() string { return n.value }

type Project struct {
	ID          kernel.ProjectID
	WorkspaceID kernel.WorkspaceID
	Name        ProjectName
	Description string
	Status      ProjectStatus
	CreatedBy   kernel.MemberID
	CreatedAt   time.Time

	events []events.Event
}

func NewProject(wsID kernel.WorkspaceID, name ProjectName, createdBy kernel.MemberID, desc string) *Project {
	p := &Project{
		ID:          kernel.NewProjectID(),
		WorkspaceID: wsID,
		Name:        name,
		Description: strings.TrimSpace(desc),
		Status:      ProjectStatusActive,
		CreatedBy:   createdBy,
		CreatedAt:   time.Now().UTC(),
	}
	p.record(event.NewProjectCreated(p.ID, p.WorkspaceID, p.CreatedBy))
	return p
}

func (p *Project) Archive() error {
	if p.Status == ProjectStatusArchived {
		return domain.ErrProjectAlreadyArchived
	}
	p.Status = ProjectStatusArchived
	return nil
}

func (p *Project) Rename(name ProjectName) error {
	if p.Status != ProjectStatusActive {
		return domain.ErrProjectNotActive
	}
	p.Name = name
	return nil
}

func (p *Project) UpdateDetails(name ProjectName, description string) error {
	if p.Status != ProjectStatusActive {
		return domain.ErrProjectNotActive
	}
	p.Name = name
	p.Description = strings.TrimSpace(description)
	return nil
}

func (p *Project) IsActive() bool { return p.Status == ProjectStatusActive }

func (p *Project) PullEvents() []events.Event {
	out := append([]events.Event(nil), p.events...)
	p.events = nil
	return out
}

func (p *Project) record(e events.Event) { p.events = append(p.events, e) }
