package model

import (
	"strings"
	"time"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/workspace_app/domain"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

type DockerfileStatus string

const (
	DockerfileStatusActive   DockerfileStatus = "active"
	DockerfileStatusArchived DockerfileStatus = "archived"
)

type DockerfileName struct {
	value string
}

func NewDockerfileName(raw string) (DockerfileName, error) {
	v := strings.TrimSpace(raw)
	if v == "" || len(v) > 255 {
		return DockerfileName{}, domain.ErrInvalidDockerfileName
	}
	return DockerfileName{value: v}, nil
}

func (n DockerfileName) String() string { return n.value }

type Dockerfile struct {
	ID            kernel.DockerfileID
	WorkspaceID   kernel.WorkspaceID
	Name          DockerfileName
	Language      string
	Description   string
	Detail        string
	Dockerfile    string
	Version       string
	FullPath      string
	Sha256        string
	DownloadCount int
	Score         int
	Status        DockerfileStatus
	CreatedBy     kernel.MemberID
	CreatedAt     time.Time
}

func NewDockerfile(wsID kernel.WorkspaceID, name DockerfileName, language, description, detail, dockerfile, version string, createdBy kernel.MemberID) *Dockerfile {
	return &Dockerfile{
		ID:          kernel.NewDockerfileID(),
		WorkspaceID: wsID,
		Name:        name,
		Language:    strings.TrimSpace(language),
		Description: strings.TrimSpace(description),
		Detail:      detail,
		Dockerfile:  dockerfile,
		Version:     strings.TrimSpace(version),
		Status:      DockerfileStatusActive,
		CreatedBy:   createdBy,
		CreatedAt:   time.Now().UTC(),
	}
}
