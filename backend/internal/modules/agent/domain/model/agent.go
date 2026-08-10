package model

import (
	"strings"
	"time"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/agent/domain"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/agent/domain/event"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/events"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

type AgentStatus string

const (
	AgentStatusDraft     AgentStatus = "draft"
	AgentStatusPublished AgentStatus = "published"
	AgentStatusArchived  AgentStatus = "archived"
)

type AgentName struct {
	value string
}

func NewAgentName(raw string) (AgentName, error) {
	v := strings.TrimSpace(raw)
	if v == "" || len(v) > 255 {
		return AgentName{}, domain.ErrInvalidAgentName
	}
	return AgentName{value: v}, nil
}

func (n AgentName) String() string { return n.value }

type ModelConfig struct {
	Temperature float64 `json:"temperature"`
	MaxTokens   int     `json:"max_tokens"`
}

func DefaultModelConfig() ModelConfig {
	return ModelConfig{Temperature: 0.7, MaxTokens: 4096}
}

type AgentVersion struct {
	ID                kernel.AgentVersionID
	AgentID           kernel.AgentID
	Version           int
	SystemPrompt      string
	ModelName         string
	ModelConfig       ModelConfig
	ToolIDs           []string
	KnowledgeBaseIDs  []string
	CreatedAt         time.Time
}

type Agent struct {
	ID               kernel.AgentID
	ProjectID        kernel.ProjectID
	Name             AgentName
	Status           AgentStatus
	CurrentVersionID *kernel.AgentVersionID
	CreatedAt        time.Time
	versions         []*AgentVersion
	events           []events.Event
}

func NewAgent(projectID kernel.ProjectID, name AgentName) *Agent {
	return &Agent{
		ID:        kernel.NewAgentID(),
		ProjectID: projectID,
		Name:      name,
		Status:    AgentStatusDraft,
		CreatedAt: time.Now().UTC(),
	}
}

func (a *Agent) AddVersion(prompt, modelName string, cfg ModelConfig, tools, knowledgeBaseIDs []string) (*AgentVersion, error) {
	if a.Status == AgentStatusArchived {
		return nil, domain.ErrAgentArchived
	}
	next := len(a.versions) + 1
	ver := &AgentVersion{
		ID:               kernel.NewAgentVersionID(),
		AgentID:          a.ID,
		Version:          next,
		SystemPrompt:     prompt,
		ModelName:        modelName,
		ModelConfig:      cfg,
		ToolIDs:          append([]string(nil), tools...),
		KnowledgeBaseIDs: append([]string(nil), knowledgeBaseIDs...),
		CreatedAt:        time.Now().UTC(),
	}
	a.versions = append(a.versions, ver)
	return ver, nil
}

func (a *Agent) Publish(versionID kernel.AgentVersionID) (event.AgentPublished, error) {
	if a.Status == AgentStatusArchived {
		return event.AgentPublished{}, domain.ErrAgentArchived
	}
	ver := a.findVersion(versionID)
	if ver == nil {
		return event.AgentPublished{}, domain.ErrVersionNotFound
	}
	a.Status = AgentStatusPublished
	a.CurrentVersionID = &ver.ID
	ev := event.NewAgentPublished(a.ID, a.ProjectID, ver.ID, ver.Version)
	a.record(ev)
	return ev, nil
}

func (a *Agent) Archive() error {
	if a.Status == AgentStatusArchived {
		return domain.ErrAgentArchived
	}
	a.Status = AgentStatusArchived
	return nil
}

func (a *Agent) Rename(name AgentName) error {
	if a.Status == AgentStatusArchived {
		return domain.ErrAgentArchived
	}
	a.Name = name
	return nil
}

func (a *Agent) PublishedVersion() (*AgentVersion, error) {
	if a.CurrentVersionID == nil {
		return nil, domain.ErrAgentNotPublished
	}
	ver := a.findVersion(*a.CurrentVersionID)
	if ver == nil {
		return nil, domain.ErrVersionNotFound
	}
	return ver, nil
}

func (a *Agent) Versions() []*AgentVersion {
	return append([]*AgentVersion(nil), a.versions...)
}

func (a *Agent) PullEvents() []events.Event {
	out := append([]events.Event(nil), a.events...)
	a.events = nil
	return out
}

func (a *Agent) SetVersions(versions []*AgentVersion) {
	a.versions = append([]*AgentVersion(nil), versions...)
}

func (a *Agent) findVersion(id kernel.AgentVersionID) *AgentVersion {
	for _, v := range a.versions {
		if v.ID == id {
			return v
		}
	}
	return nil
}

func (a *Agent) record(e events.Event) { a.events = append(a.events, e) }
