package model

type WorkflowStage struct {
	Name     string        `json:"name"`
	Parallel bool          `json:"parallel"`
	ExecMode string        `json:"exec_mode,omitempty"`
	Jobs     []WorkflowJob `json:"jobs"`
}

const (
	StageExecAuto   = "auto"
	StageExecManual = "manual"
)

func NormalizeStageExecMode(mode string) string {
	if mode == StageExecManual {
		return StageExecManual
	}
	return StageExecAuto
}

type WorkflowJob struct {
	Name string         `json:"name"`
	Type string         `json:"type"`
	Spec map[string]any `json:"spec"`
}

type ProjectBuildSpec struct {
	ProjectID  string `json:"project_id"`
	Branch     string `json:"branch"`
	GitRefType string `json:"git_ref_type,omitempty"`
}

const (
	JobTypeSparkopsBuild           = "sparkops-build"
	JobTypeSparkopsCreateVersion   = "sparkops-create-version"
	JobTypeSparkopsDeploy          = "sparkops-deploy"
	JobTypeSparkopsDeployVersion   = "sparkops-deploy-version"
	JobTypeSparkopsEnvironmentSync = "sparkops-environment-sync"
	JobTypeSparkopsApproval        = "sparkops-approval"
	JobTypeFreestyle               = "freestyle"
	JobTypeNotify                  = "notify"
)

const (
	ApprovalStatusPending  = "pending"
	ApprovalStatusApproved = "approved"
	ApprovalStatusRejected = "rejected"
)
