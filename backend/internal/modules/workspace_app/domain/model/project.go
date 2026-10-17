package model

import (
	"encoding/json"
	"reflect"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/workspace_app/domain"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/workspace_app/domain/event"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/events"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

type ProjectStatus string

const (
	ProjectStatusActive    ProjectStatus = "active"
	ProjectStatusArchiving ProjectStatus = "archiving"
	ProjectStatusArchived  ProjectStatus = "archived"
)

type ProjectReviewStatus string

const (
	ProjectReviewNone     ProjectReviewStatus = "none"
	ProjectReviewPending  ProjectReviewStatus = "pending"
	ProjectReviewApproved ProjectReviewStatus = "approved"
	ProjectReviewRejected ProjectReviewStatus = "rejected"
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
	CPURequest            string            `json:"cpu_request,omitempty"`
	CPULimit              string            `json:"cpu_limit,omitempty"`
	MemoryRequest         string            `json:"memory_request,omitempty"`
	MemoryLimit           string            `json:"memory_limit,omitempty"`
	Replicas              int               `json:"replicas,omitempty"`
	Ports                 []ProjectPort     `json:"ports,omitempty"`
	Command               string            `json:"command,omitempty"`
	Args                  string            `json:"args,omitempty"`
	ProjectConfig         string            `json:"project_config,omitempty"`
	ServiceTemplateID     string            `json:"service_template_id,omitempty"`
	DeploymentTemplateID  string            `json:"deployment_template_id,omitempty"`
	JenkinsfileTemplateID string            `json:"jenkinsfile_template_id,omitempty"`
	RegistryType          string            `json:"registry_type,omitempty"`
	RegistrySecretID      string            `json:"registry_secret_id,omitempty"`
	SonarqubeSecretID     string            `json:"sonarqube_secret_id,omitempty"`
	EnableStaticScan      bool              `json:"enable_static_scan,omitempty"`
	ManifestOverrides     map[string]string `json:"manifest_overrides,omitempty"`
}

type Project struct {
	ID                 kernel.ProjectID
	WorkspaceID        kernel.WorkspaceID
	ProductID          *kernel.ProductID
	Name               ProjectName
	DisplayName        string
	Description        string
	Namespace          string
	Language           string
	GitURL             string
	SecretID           string
	GitRefType         string
	GitRefValue        string
	DockerfilePath     string
	TemplateID         string
	DefaultClusterID   string
	JenkinsSecretID    string
	KubernetesSecretID string
	Resources          ProjectResources
	Status             ProjectStatus
	ArchiveGeneration  string
	ReviewStatus       ProjectReviewStatus
	SubmittedBy        *kernel.MemberID
	SubmittedAt        *time.Time
	ReviewedBy         *kernel.MemberID
	ReviewedAt         *time.Time
	ReviewComment      string
	CreatedBy          kernel.MemberID
	CreatedAt          time.Time

	events []events.Event
}

type ProductIDPatch struct {
	Set   bool
	Value *kernel.ProductID
}

type CreateProjectInput struct {
	ProductID          *kernel.ProductID
	Name               string
	DisplayName        string
	Description        string
	Namespace          string
	Language           string
	GitURL             string
	SecretID           string
	GitRefType         string
	GitRefValue        string
	DockerfilePath     string
	TemplateID         string
	DefaultClusterID   string
	JenkinsSecretID    string
	KubernetesSecretID string
	Resources          ProjectResources
}

type UpdateProjectInput struct {
	ProductID          ProductIDPatch
	Name               *string
	DisplayName        *string
	Description        *string
	Namespace          *string
	Language           *string
	GitURL             *string
	SecretID           *string
	GitRefType         *string
	GitRefValue        *string
	DockerfilePath     *string
	TemplateID         *string
	DefaultClusterID   *string
	JenkinsSecretID    *string
	KubernetesSecretID *string
	Resources          *ProjectResources
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
		ID:                 kernel.NewProjectID(),
		WorkspaceID:        wsID,
		ProductID:          input.ProductID,
		Name:               name,
		DisplayName:        strings.TrimSpace(input.DisplayName),
		Description:        strings.TrimSpace(input.Description),
		Namespace:          strings.TrimSpace(input.Namespace),
		Language:           strings.TrimSpace(input.Language),
		GitURL:             strings.TrimSpace(input.GitURL),
		SecretID:           strings.TrimSpace(input.SecretID),
		GitRefType:         strings.TrimSpace(input.GitRefType),
		GitRefValue:        strings.TrimSpace(input.GitRefValue),
		DockerfilePath:     dockerfile,
		TemplateID:         strings.TrimSpace(input.TemplateID),
		DefaultClusterID:   strings.TrimSpace(input.DefaultClusterID),
		JenkinsSecretID:    strings.TrimSpace(input.JenkinsSecretID),
		KubernetesSecretID: strings.TrimSpace(input.KubernetesSecretID),
		Resources:          resources,
		Status:             ProjectStatusActive,
		ReviewStatus:       ProjectReviewNone,
		CreatedBy:          createdBy,
		CreatedAt:          time.Now().UTC(),
	}
	p.record(event.NewProjectCreated(p.ID, p.WorkspaceID, p.CreatedBy))
	return p
}

func (p *Project) BeginArchiving() error {
	if p.Status != ProjectStatusActive {
		return domain.ErrProjectNotActive
	}
	p.Status = ProjectStatusArchiving
	p.ArchiveGeneration = uuid.NewString()
	return nil
}

func (p *Project) AbortArchiving() error {
	if p.Status != ProjectStatusArchiving {
		return domain.ErrProjectNotArchiving
	}
	p.Status = ProjectStatusActive
	p.ArchiveGeneration = ""
	return nil
}

func (p *Project) IsArchiving() bool { return p.Status == ProjectStatusArchiving }

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
	return p.ApplyPatch(UpdateProjectInput{
		Name: &input.Name, DisplayName: &input.DisplayName, Description: &input.Description,
		Namespace: &input.Namespace, Language: &input.Language, GitURL: &input.GitURL,
		SecretID: &input.SecretID, GitRefType: &input.GitRefType, GitRefValue: &input.GitRefValue,
		DockerfilePath: &input.DockerfilePath, TemplateID: &input.TemplateID, Resources: &input.Resources,
		DefaultClusterID: &input.DefaultClusterID, JenkinsSecretID: &input.JenkinsSecretID,
		KubernetesSecretID: &input.KubernetesSecretID,
	})
}

func (p *Project) ApplyPatch(input UpdateProjectInput) error {
	if p.Status != ProjectStatusActive {
		return domain.ErrProjectNotActive
	}
	configChanged := p.patchChangesConfiguration(input)
	if p.ReviewStatus == ProjectReviewPending && configChanged {
		return domain.ErrProjectReviewLocked
	}
	if input.ProductID.Set {
		p.ProductID = input.ProductID.Value
	}
	if input.Name != nil {
		name, err := NewProjectName(*input.Name)
		if err != nil {
			return err
		}
		p.Name = name
	}
	if input.DisplayName != nil {
		p.DisplayName = strings.TrimSpace(*input.DisplayName)
	}
	if input.Description != nil {
		p.Description = strings.TrimSpace(*input.Description)
	}
	if input.Namespace != nil {
		p.Namespace = strings.TrimSpace(*input.Namespace)
	}
	if input.Language != nil {
		p.Language = strings.TrimSpace(*input.Language)
	}
	if input.GitURL != nil {
		p.GitURL = strings.TrimSpace(*input.GitURL)
	}
	if input.SecretID != nil {
		p.SecretID = strings.TrimSpace(*input.SecretID)
	}
	if input.GitRefType != nil {
		p.GitRefType = strings.TrimSpace(*input.GitRefType)
	}
	if input.GitRefValue != nil {
		p.GitRefValue = strings.TrimSpace(*input.GitRefValue)
	}
	if input.DockerfilePath != nil {
		dockerfile := strings.TrimSpace(*input.DockerfilePath)
		if dockerfile == "" {
			dockerfile = "./Dockerfile"
		}
		p.DockerfilePath = dockerfile
	}
	if input.TemplateID != nil {
		p.TemplateID = strings.TrimSpace(*input.TemplateID)
	}
	if input.DefaultClusterID != nil {
		p.DefaultClusterID = strings.TrimSpace(*input.DefaultClusterID)
	}
	if input.JenkinsSecretID != nil {
		p.JenkinsSecretID = strings.TrimSpace(*input.JenkinsSecretID)
	}
	if input.KubernetesSecretID != nil {
		p.KubernetesSecretID = strings.TrimSpace(*input.KubernetesSecretID)
	}
	if input.Resources != nil {
		p.Resources = mergeProjectResources(p.Resources, *input.Resources)
	}
	if p.ReviewStatus == ProjectReviewApproved && configChanged {
		p.resetReview()
	}
	return nil
}

func mergeProjectResources(existing, incoming ProjectResources) ProjectResources {
	out := incoming
	if out.Replicas <= 0 {
		if existing.Replicas > 0 {
			out.Replicas = existing.Replicas
		} else {
			out.Replicas = 1
		}
	}
	if strings.TrimSpace(out.ServiceTemplateID) == "" {
		out.ServiceTemplateID = existing.ServiceTemplateID
	}
	if strings.TrimSpace(out.DeploymentTemplateID) == "" {
		out.DeploymentTemplateID = existing.DeploymentTemplateID
	}
	if strings.TrimSpace(out.JenkinsfileTemplateID) == "" {
		out.JenkinsfileTemplateID = existing.JenkinsfileTemplateID
	}
	if incoming.ManifestOverrides == nil {
		out.ManifestOverrides = existing.ManifestOverrides
	} else {
		merged := make(map[string]string, len(existing.ManifestOverrides)+len(incoming.ManifestOverrides))
		for k, v := range existing.ManifestOverrides {
			merged[k] = v
		}
		for k, v := range incoming.ManifestOverrides {
			merged[k] = v
		}
		out.ManifestOverrides = merged
	}
	return out
}

func NormalizeManifestKind(kind string) string {
	switch strings.ToLower(strings.TrimSpace(kind)) {
	case "service":
		return "Service"
	case "deployment":
		return "Deployment"
	case "jenkinsfile":
		return "jenkinsFile"
	default:
		return ""
	}
}

func (p *Project) SetManifestOverride(kind, content string) error {
	if p.Status != ProjectStatusActive {
		return domain.ErrProjectNotActive
	}
	if p.ReviewStatus == ProjectReviewPending {
		return domain.ErrProjectReviewLocked
	}
	normalized := NormalizeManifestKind(kind)
	if normalized == "" {
		return domain.ErrInvalidManifestKind
	}
	content = strings.TrimSpace(content)
	if content == "" {
		return domain.ErrInvalidManifestContent
	}
	if p.Resources.ManifestOverrides == nil {
		p.Resources.ManifestOverrides = map[string]string{}
	}
	p.Resources.ManifestOverrides[normalized] = content
	if !strings.HasSuffix(p.Resources.ManifestOverrides[normalized], "\n") {
		p.Resources.ManifestOverrides[normalized] += "\n"
	}
	if p.ReviewStatus == ProjectReviewApproved {
		p.resetReview()
	}
	return nil
}

func (p *Project) IsConfigComplete() bool {
	hasTemplate := strings.TrimSpace(p.TemplateID) != "" ||
		strings.TrimSpace(p.Resources.ServiceTemplateID) != "" ||
		strings.TrimSpace(p.Resources.DeploymentTemplateID) != "" ||
		strings.TrimSpace(p.Resources.JenkinsfileTemplateID) != ""
	dockerfile := strings.TrimSpace(p.DockerfilePath)
	if dockerfile == "" {
		dockerfile = "./Dockerfile"
	}
	return strings.TrimSpace(p.GitURL) != "" &&
		strings.TrimSpace(p.GitRefValue) != "" &&
		dockerfile != "" &&
		hasTemplate
}

func (p *Project) SubmitReview(actor kernel.MemberID) error {
	if p.Status != ProjectStatusActive {
		return domain.ErrProjectNotActive
	}
	if !p.IsConfigComplete() {
		return domain.ErrProjectConfigIncomplete
	}
	if p.ReviewStatus != ProjectReviewNone && p.ReviewStatus != ProjectReviewRejected {
		return domain.ErrProjectReviewInvalidState
	}
	now := time.Now().UTC()
	p.ReviewStatus = ProjectReviewPending
	p.SubmittedBy = &actor
	p.SubmittedAt = &now
	p.ReviewedBy = nil
	p.ReviewedAt = nil
	p.ReviewComment = ""
	return nil
}

func (p *Project) DecideReview(actor kernel.MemberID, decision ProjectReviewStatus, comment string) error {
	if p.Status != ProjectStatusActive {
		return domain.ErrProjectNotActive
	}
	if p.ReviewStatus != ProjectReviewPending {
		return domain.ErrProjectReviewInvalidState
	}
	if p.SubmittedBy != nil && *p.SubmittedBy == actor {
		return domain.ErrProjectReviewSelf
	}
	if decision != ProjectReviewApproved && decision != ProjectReviewRejected {
		return domain.ErrProjectReviewInvalidState
	}
	comment = strings.TrimSpace(comment)
	if decision == ProjectReviewRejected && comment == "" {
		return domain.ErrProjectReviewCommentRequired
	}
	now := time.Now().UTC()
	p.ReviewStatus = decision
	p.ReviewedBy = &actor
	p.ReviewedAt = &now
	p.ReviewComment = comment
	return nil
}

func (p *Project) patchChangesConfiguration(input UpdateProjectInput) bool {
	if input.Namespace != nil && p.Namespace != strings.TrimSpace(*input.Namespace) {
		return true
	}
	if input.Language != nil && p.Language != strings.TrimSpace(*input.Language) {
		return true
	}
	if input.GitURL != nil && p.GitURL != strings.TrimSpace(*input.GitURL) {
		return true
	}
	if input.SecretID != nil && p.SecretID != strings.TrimSpace(*input.SecretID) {
		return true
	}
	if input.GitRefType != nil && p.GitRefType != strings.TrimSpace(*input.GitRefType) {
		return true
	}
	if input.GitRefValue != nil && p.GitRefValue != strings.TrimSpace(*input.GitRefValue) {
		return true
	}
	if input.DockerfilePath != nil {
		dockerfile := strings.TrimSpace(*input.DockerfilePath)
		if dockerfile == "" {
			dockerfile = "./Dockerfile"
		}
		if p.DockerfilePath != dockerfile {
			return true
		}
	}
	if input.TemplateID != nil && p.TemplateID != strings.TrimSpace(*input.TemplateID) {
		return true
	}
	if input.DefaultClusterID != nil && p.DefaultClusterID != strings.TrimSpace(*input.DefaultClusterID) {
		return true
	}
	if input.JenkinsSecretID != nil && p.JenkinsSecretID != strings.TrimSpace(*input.JenkinsSecretID) {
		return true
	}
	if input.KubernetesSecretID != nil && p.KubernetesSecretID != strings.TrimSpace(*input.KubernetesSecretID) {
		return true
	}
	if input.Resources != nil {
		resources := *input.Resources
		if resources.Replicas <= 0 {
			resources.Replicas = 1
		}
		if !reflect.DeepEqual(p.Resources, resources) {
			return true
		}
	}
	return false
}

func (p *Project) resetReview() {
	p.ReviewStatus = ProjectReviewNone
	p.SubmittedBy = nil
	p.SubmittedAt = nil
	p.ReviewedBy = nil
	p.ReviewedAt = nil
	p.ReviewComment = ""
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
