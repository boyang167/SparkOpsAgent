package model

import (
	"time"

	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

const (
	ProjectRoleLead   = "lead"
	ProjectRoleMember = "member"
	ProjectRoleViewer = "viewer"
)

type ProjectMember struct {
	ProjectID kernel.ProjectID
	MemberID  kernel.MemberID
	Role      string
	JoinedAt  time.Time
}

func NewProjectMember(projectID kernel.ProjectID, memberID kernel.MemberID, role string) *ProjectMember {
	if role == "" {
		role = ProjectRoleMember
	}
	return &ProjectMember{
		ProjectID: projectID,
		MemberID:  memberID,
		Role:      role,
		JoinedAt:  time.Now().UTC(),
	}
}
