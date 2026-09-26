package model

import (
	"crypto/sha256"
	"encoding/hex"
	"time"

	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

type ProjectVersionStatus string

const (
	ProjectVersionStatusCreating ProjectVersionStatus = "creating"
	ProjectVersionStatusReady    ProjectVersionStatus = "ready"
	ProjectVersionStatusFailed   ProjectVersionStatus = "failed"
)

type VersionManifest struct {
	Kind    string `json:"kind"`
	Content string `json:"content"`
	Hash    string `json:"hash,omitempty"`
}

type VersionImage struct {
	ServiceName string `json:"service_name"`
	Image       string `json:"image"`
	Tag         string `json:"tag"`
	BuildID     string `json:"build_id,omitempty"`
}

type ProjectVersion struct {
	ID              kernel.ProjectVersionID
	WorkspaceID     kernel.WorkspaceID
	ProjectID       kernel.ProjectID
	ProductID       *kernel.ProductID
	VersionTag      string
	Status          ProjectVersionStatus
	Source          string
	WorkflowRunID   *kernel.ProductWorkflowRunID
	WorkflowJobName string
	GitRefType      string
	GitRefValue     string
	Manifests       []VersionManifest
	Images          []VersionImage
	ErrorMessage    string
	CreatedBy       kernel.MemberID
	CreatedAt       time.Time
}

func HashManifestContent(content string) string {
	sum := sha256.Sum256([]byte(content))
	return hex.EncodeToString(sum[:8])
}

func NewProjectVersion(
	wsID kernel.WorkspaceID,
	projectID kernel.ProjectID,
	productID *kernel.ProductID,
	versionTag string,
	workflowRunID kernel.ProductWorkflowRunID,
	workflowJobName string,
	gitRefType, gitRefValue string,
	manifests []VersionManifest,
	images []VersionImage,
	createdBy kernel.MemberID,
) *ProjectVersion {
	now := time.Now().UTC()
	return &ProjectVersion{
		ID: kernel.NewProjectVersionID(), WorkspaceID: wsID, ProjectID: projectID,
		ProductID: productID, VersionTag: versionTag, Status: ProjectVersionStatusReady,
		Source: "workflow", WorkflowRunID: &workflowRunID, WorkflowJobName: workflowJobName,
		GitRefType: gitRefType, GitRefValue: gitRefValue,
		Manifests: manifests, Images: images, CreatedBy: createdBy, CreatedAt: now,
	}
}
