package model

type WorkflowStage struct {
	Name     string        `json:"name"`
	Parallel bool          `json:"parallel"`
	Jobs     []WorkflowJob `json:"jobs"`
}

type WorkflowJob struct {
	Name string         `json:"name"`
	Type string         `json:"type"`
	Spec map[string]any `json:"spec"`
}

type ProjectBuildSpec struct {
	ProjectID string `json:"project_id"`
	Branch    string `json:"branch"`
}

const (
	JobTypeSparkopsBuild     = "sparkops-build"
	JobTypeSparkopsDeploy    = "sparkops-deploy"
	JobTypeSparkopsApproval  = "sparkops-approval"
)

const (
	ApprovalStatusPending  = "pending"
	ApprovalStatusApproved = "approved"
	ApprovalStatusRejected = "rejected"
)
