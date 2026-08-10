package model

import (
	"strings"
	"time"

	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

type BuildStatus string

const (
	BuildStatusRunning  BuildStatus = "running"
	BuildStatusSuccess  BuildStatus = "success"
	BuildStatusFailed   BuildStatus = "failed"
	BuildStatusAborted  BuildStatus = "aborted"
)

type Build struct {
	ID                kernel.BuildID
	ProjectID         kernel.ProjectID
	MemberID          kernel.MemberID
	WorkspaceID       kernel.WorkspaceID
	Status            BuildStatus
	Branch            string
	BuildNumber       int
	JenkinsJob        string
	ConsoleLog        string
	Error             string
	RollbackOfBuildID string
	StartedAt         time.Time
	CompletedAt       *time.Time
}

func NewBuild(
	projectID kernel.ProjectID,
	memberID kernel.MemberID,
	wsID kernel.WorkspaceID,
	branch string,
) *Build {
	if strings.TrimSpace(branch) == "" {
		branch = "main"
	}
	return &Build{
		ID:          kernel.NewBuildID(),
		ProjectID:   projectID,
		MemberID:    memberID,
		WorkspaceID: wsID,
		Status:      BuildStatusRunning,
		Branch:      branch,
		StartedAt:   time.Now().UTC(),
	}
}

func (b *Build) Succeed(buildNumber int, jenkinsJob, consoleLog string) {
	now := time.Now().UTC()
	b.Status = BuildStatusSuccess
	b.BuildNumber = buildNumber
	b.JenkinsJob = jenkinsJob
	b.ConsoleLog = consoleLog
	b.CompletedAt = &now
}

func (b *Build) Fail(errMsg, consoleLog string) {
	now := time.Now().UTC()
	b.Status = BuildStatusFailed
	b.Error = errMsg
	b.ConsoleLog = consoleLog
	b.CompletedAt = &now
}

func (b *Build) Abort(consoleLog string) {
	now := time.Now().UTC()
	b.Status = BuildStatusAborted
	b.ConsoleLog = consoleLog
	b.Error = "build aborted"
	b.CompletedAt = &now
}

func (b *Build) IsRunning() bool { return b.Status == BuildStatusRunning }
