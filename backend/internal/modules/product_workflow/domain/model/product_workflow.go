package model

import (
	"encoding/json"
	"time"

	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

type WorkflowStatus string

const (
	WorkflowStatusDraft  WorkflowStatus = "draft"
	WorkflowStatusActive WorkflowStatus = "active"
)

type ProductWorkflow struct {
	ID              kernel.ProductWorkflowID
	ProductID       kernel.ProductID
	WorkspaceID     kernel.WorkspaceID
	Name            string
	Key             string
	Description     string
	Status          WorkflowStatus
	Stages          []WorkflowStage
	TemplateBinding json.RawMessage
	Origin          json.RawMessage
	CreatedBy       kernel.MemberID
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

type RunStatus string

const (
	RunStatusPending RunStatus = "pending"
	RunStatusRunning RunStatus = "running"
	RunStatusPaused  RunStatus = "paused"
	RunStatusSuccess RunStatus = "success"
	RunStatusFailed  RunStatus = "failed"
)

type JobRunStatus string

const (
	JobRunStatusPending JobRunStatus = "pending"
	JobRunStatusRunning JobRunStatus = "running"
	JobRunStatusPaused  JobRunStatus = "paused"
	JobRunStatusSuccess JobRunStatus = "success"
	JobRunStatusFailed  JobRunStatus = "failed"
	JobRunStatusSkipped JobRunStatus = "skipped"
)

type ProjectBuildRun struct {
	ProjectID    string       `json:"project_id"`
	Branch       string       `json:"branch"`
	BuildID      string       `json:"build_id,omitempty"`
	Status       JobRunStatus `json:"status"`
	ErrorMessage string       `json:"error_message,omitempty"`
}

type ProjectDeployRun struct {
	ProjectID       string       `json:"project_id"`
	EnvironmentID   string       `json:"environment_id,omitempty"`
	DeploymentRunID string       `json:"deployment_run_id,omitempty"`
	ImageTag        string       `json:"image_tag,omitempty"`
	Status          JobRunStatus `json:"status"`
	ErrorMessage    string       `json:"error_message,omitempty"`
}

type ApprovalRun struct {
	Status  string `json:"status"`
	ActorID string `json:"actor_id,omitempty"`
	Message string `json:"message,omitempty"`
}

type JobRun struct {
	Name         string             `json:"name"`
	Type         string             `json:"type"`
	Status       JobRunStatus       `json:"status"`
	Builds       []ProjectBuildRun  `json:"builds,omitempty"`
	Deploys      []ProjectDeployRun `json:"deploys,omitempty"`
	Approval     *ApprovalRun       `json:"approval,omitempty"`
	Message      string             `json:"message,omitempty"`
	ErrorMessage string             `json:"error_message,omitempty"`
}

type StageRun struct {
	Name            string       `json:"name"`
	Status          JobRunStatus `json:"status"`
	ExecMode        string       `json:"exec_mode,omitempty"`
	ManualConfirmed bool         `json:"manual_confirmed,omitempty"`
	Jobs            []JobRun     `json:"jobs"`
}

type ProductWorkflowRun struct {
	ID           kernel.ProductWorkflowRunID
	WorkflowID   kernel.ProductWorkflowID
	ProductID    kernel.ProductID
	Status       RunStatus
	Trigger      json.RawMessage
	Input        json.RawMessage
	StageRuns    []StageRun
	CurrentStage int
	ErrorMessage string
	StartedAt    *time.Time
	CompletedAt  *time.Time
	CreatedAt    time.Time
}

type WorkflowBuildRef struct {
	BuildID   kernel.BuildID
	RunID     kernel.ProductWorkflowRunID
	JobName   string
	ProjectID kernel.ProjectID
}

type WorkflowDeployRef struct {
	DeploymentRunID kernel.DeploymentRunID
	RunID           kernel.ProductWorkflowRunID
	JobName         string
	ProjectID       kernel.ProjectID
}

func NewProductWorkflow(productID kernel.ProductID, wsID kernel.WorkspaceID, name, key string, stages []WorkflowStage, createdBy kernel.MemberID) *ProductWorkflow {
	now := time.Now().UTC()
	return &ProductWorkflow{
		ID: kernel.NewProductWorkflowID(), ProductID: productID, WorkspaceID: wsID,
		Name: name, Key: key, Status: WorkflowStatusActive, Stages: stages,
		Origin: json.RawMessage(`{"kind":"from_template"}`), CreatedBy: createdBy,
		CreatedAt: now, UpdatedAt: now,
	}
}

func NewBlankProductWorkflow(productID kernel.ProductID, wsID kernel.WorkspaceID, name, key, description string, stages []WorkflowStage, createdBy kernel.MemberID) *ProductWorkflow {
	if len(stages) == 0 {
		stages = []WorkflowStage{{Name: "构建", Parallel: false, Jobs: []WorkflowJob{}}}
	}
	now := time.Now().UTC()
	return &ProductWorkflow{
		ID: kernel.NewProductWorkflowID(), ProductID: productID, WorkspaceID: wsID,
		Name: name, Key: key, Description: description, Status: WorkflowStatusDraft, Stages: stages,
		Origin: json.RawMessage(`{"kind":"custom"}`), CreatedBy: createdBy,
		CreatedAt: now, UpdatedAt: now,
	}
}

func InitStageRuns(stages []WorkflowStage) []StageRun {
	out := make([]StageRun, 0, len(stages))
	for _, stage := range stages {
		jobs := make([]JobRun, 0, len(stage.Jobs))
		for _, job := range stage.Jobs {
			jobs = append(jobs, JobRun{Name: job.Name, Type: job.Type, Status: JobRunStatusPending, Builds: nil})
		}
		out = append(out, StageRun{
			Name: stage.Name, Status: JobRunStatusPending, ExecMode: NormalizeStageExecMode(stage.ExecMode), Jobs: jobs,
		})
	}
	return out
}

func NewProductWorkflowRun(workflowID kernel.ProductWorkflowID, productID kernel.ProductID, stages []WorkflowStage) *ProductWorkflowRun {
	now := time.Now().UTC()
	return &ProductWorkflowRun{
		ID: kernel.NewProductWorkflowRunID(), WorkflowID: workflowID, ProductID: productID,
		Status: RunStatusPending, StageRuns: InitStageRuns(stages), CreatedAt: now,
	}
}
