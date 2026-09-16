package model

import (
	"encoding/json"
	"time"

	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

type EnvironmentType string

const (
	EnvironmentTypeTest       EnvironmentType = "test"
	EnvironmentTypeProduction EnvironmentType = "production"
)

type EnvironmentStatus string

const (
	EnvironmentStatusActive   EnvironmentStatus = "active"
	EnvironmentStatusDisabled EnvironmentStatus = "disabled"
)

type Environment struct {
	ID          kernel.EnvironmentID
	ProductID   kernel.ProductID
	WorkspaceID kernel.WorkspaceID
	Name        string
	EnvType     EnvironmentType
	ClusterID   kernel.ClusterInfoID
	Namespace   string
	Variables   map[string]string
	Status      EnvironmentStatus
	CreatedBy   kernel.MemberID
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

type EnvironmentProject struct {
	EnvironmentID           kernel.EnvironmentID
	ProjectID               kernel.ProjectID
	Enabled                 bool
	CurrentImageTag         string
	CurrentDeploymentRunID  *kernel.DeploymentRunID
	LastDeployedAt          *time.Time
}

func NewEnvironment(
	productID kernel.ProductID,
	wsID kernel.WorkspaceID,
	name, namespace string,
	envType EnvironmentType,
	clusterID kernel.ClusterInfoID,
	variables map[string]string,
	createdBy kernel.MemberID,
) *Environment {
	now := time.Now().UTC()
	if variables == nil {
		variables = map[string]string{}
	}
	if envType == "" {
		envType = EnvironmentTypeTest
	}
	return &Environment{
		ID: kernel.NewEnvironmentID(), ProductID: productID, WorkspaceID: wsID,
		Name: name, EnvType: envType, ClusterID: clusterID, Namespace: namespace,
		Variables: variables, Status: EnvironmentStatusActive, CreatedBy: createdBy,
		CreatedAt: now, UpdatedAt: now,
	}
}

func (e *Environment) VariablesJSON() json.RawMessage {
	raw, _ := json.Marshal(e.Variables)
	if len(raw) == 0 {
		return json.RawMessage(`{}`)
	}
	return raw
}
