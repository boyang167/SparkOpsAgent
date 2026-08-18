package model

import (
	"encoding/json"
	"regexp"
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

var projectSlugPattern = regexp.MustCompile(`^[a-z][a-z0-9-]*[a-z0-9]$|^[a-z]$`)

type ProjectName struct {
	value string
}

func NewProjectName(raw string) (ProjectName, error) {
	v := strings.TrimSpace(raw)
	if v == "" || len(v) > 255 {
		return ProjectName{}, domain.ErrInvalidProjectName
	}
	if len(v) < 2 || len(v) > 50 || !projectSlugPattern.MatchString(v) {
		return ProjectName{}, domain.ErrInvalidProjectName
	}
	return ProjectName{value: v}, nil
}

// ProjectNameFromDB loads persisted names without re-validating slug rules.
func ProjectNameFromDB(raw string) (ProjectName, error) {
	v := strings.TrimSpace(raw)
	if v == "" || len(v) > 255 {
		return ProjectName{}, domain.ErrInvalidProjectName
	}
	return ProjectName{value: v}, nil
}

func (n ProjectName) String() string { return n.value }

type ProjectPort struct {
	Name          string `json:"name,omitempty"`
	ContainerPort int    `json:"container_port"`
	ServicePort   int    `json:"service_port,omitempty"`
	Protocol      string `json:"protocol,omitempty"`
}

type ProjectResources struct {
	CPURequest             string        `json:"cpu_request,omitempty"`
	CPULimit               string        `json:"cpu_limit,omitempty"`
	MemoryRequest          string        `json:"memory_request,omitempty"`
	MemoryLimit            string        `json:"memory_limit,omitempty"`
	Replicas               int           `json:"replicas,omitempty"`
	Ports                  []ProjectPort `json:"ports,omitempty"`
	Command                string        `json:"command,omitempty"`
	Args                   string        `json:"args,omitempty"`
	ProjectConfig          string        `json:"project_config,omitempty"`
	ServiceTemplateID      string        `json:"service_template_id,omitempty"`
	DeploymentTemplateID  string        `json:"deployment_template_id,omitempty"`
	JenkinsfileTemplateID string        `json:"jenkinsfile_template_id,omitempty"`
	RegistryType           string        `json:"registry_type,omitempty"`
	RegistrySecretID      string        `json:"registry_secret_id,omitempty"`
	SonarqubeSecretID      string        `json:"sonarqube_secret_id,omitempty"`
	EnableStaticScan       bool          `json:"enable_static_scan,omitempty"`
}

type Project struct {
	ID             kernel.ProjectID
	WorkspaceID    kernel.WorkspaceID
	Name           ProjectName
	DisplayName    string
	Description    string
	Namespace      string
	Language       string
	GitURL         string
	SecretID       string
	GitRefType     string
	GitRefValue    string
	DockerfilePath string
	TemplateID     string
	Resources      ProjectResources
	Status         ProjectStatus
	CreatedBy      kernel.MemberID
	CreatedAt      time.Time

	events []events.Event
}

type CreateProjectInput struct {
	Name           string
	DisplayName    string
	Description    string
	Namespace      string
	Language       string
	GitURL         string
	SecretID       string
	GitRefType     string
	GitRefValue    string
	DockerfilePath string
	TemplateID     string
	Resources      ProjectResources
}

func NewProject(wsID kernel.WorkspaceID, name ProjectName, createdBy kernel.MemberID, input CreateProjectInput) *Project {
	dockerfile := strings.TrimSpace(input.DockerfilePath)
	if dockerfile == "" {
		dockerfile = "./Dockerfile"
	}
	resources := input.Resources
	if resources.Replicas <= 0 {
		resources.Replicas = 1
	}

	p := &Project{
		ID:             kernel.NewProjectID(),
		WorkspaceID:    wsID,
		Name:           name,
		DisplayName:    strings.TrimSpace(input.DisplayName),
		Description:    strings.TrimSpace(input.Description),
		Namespace:      strings.TrimSpace(input.Namespace),
		Language:       strings.TrimSpace(input.Language),
		GitURL:         strings.TrimSpace(input.GitURL),
		SecretID:       strings.TrimSpace(input.SecretID),
		GitRefType:     strings.TrimSpace(input.GitRefType),
		GitRefValue:    strings.TrimSpace(input.GitRefValue),
		DockerfilePath: dockerfile,
		TemplateID:     strings.TrimSpace(input.TemplateID),
		Resources:      resources,
		Status:         ProjectStatusActive,
		CreatedBy:      createdBy,
		CreatedAt:      time.Now().UTC(),
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

func (p *Project) UpdateDetails(input CreateProjectInput) error {
	if p.Status != ProjectStatusActive {
		return domain.ErrProjectNotActive
	}
	name, err := NewProjectName(input.Name)
	if err != nil {
		return err
	}
	p.Name = name
	p.DisplayName = strings.TrimSpace(input.DisplayName)
	p.Description = strings.TrimSpace(input.Description)
	p.Namespace = strings.TrimSpace(input.Namespace)
	p.Language = strings.TrimSpace(input.Language)
	p.GitURL = strings.TrimSpace(input.GitURL)
	p.SecretID = strings.TrimSpace(input.SecretID)
	p.GitRefType = strings.TrimSpace(input.GitRefType)
	p.GitRefValue = strings.TrimSpace(input.GitRefValue)
	if dockerfile := strings.TrimSpace(input.DockerfilePath); dockerfile != "" {
		p.DockerfilePath = dockerfile
	}
	p.TemplateID = strings.TrimSpace(input.TemplateID)
	resources := input.Resources
	if resources.Replicas <= 0 {
		resources.Replicas = 1
	}
	p.Resources = resources
	return nil
}

func (p *Project) IsActive() bool { return p.Status == ProjectStatusActive }

func (p *Project) PullEvents() []events.Event {
	out := append([]events.Event(nil), p.events...)
	p.events = nil
	return out
}

func (p *Project) record(e events.Event) { p.events = append(p.events, e) }

func EncodeProjectResources(r ProjectResources) ([]byte, error) {
	return json.Marshal(r)
}

func DecodeProjectResources(raw []byte) (ProjectResources, error) {
	if len(raw) == 0 {
		return ProjectResources{Replicas: 1}, nil
	}
	var r ProjectResources
	if err := json.Unmarshal(raw, &r); err != nil {
		return ProjectResources{Replicas: 1}, err
	}
	if r.Replicas <= 0 {
		r.Replicas = 1
	}
	return r, nil
}
