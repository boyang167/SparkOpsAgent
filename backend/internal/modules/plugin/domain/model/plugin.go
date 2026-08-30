package model

import (
	"errors"
	"time"

	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

type PluginStatus string

const (
	PluginStatusUploaded        PluginStatus = "uploaded"
	PluginStatusPROpen          PluginStatus = "pr_open"
	PluginStatusMerged          PluginStatus = "merged"
	PluginStatusDeployed        PluginStatus = "deployed"
	PluginStatusPRFailed        PluginStatus = "pr_failed"
	PluginStatusCIFailed        PluginStatus = "ci_failed"
	PluginStatusUninstalling    PluginStatus = "uninstalling"
	PluginStatusUninstallMerged PluginStatus = "uninstall_merged"
)

var ErrInvalidPluginStatusTransition = errors.New("invalid plugin status transition")

type PluginRecord struct {
	ID             string          `json:"id"`
	Version        string          `json:"version"`
	DisplayName    string          `json:"display_name"`
	ManifestYAML   string          `json:"manifest_yaml"`
	InstalledFiles []string        `json:"installed_files"`
	Status         PluginStatus    `json:"status"`
	PRURL          string          `json:"pr_url"`
	CreatedBy      kernel.MemberID `json:"created_by"`
	CreatedAt      time.Time       `json:"created_at"`
	UpdatedAt      time.Time       `json:"updated_at"`
}

type WorkspaceBinding struct {
	WorkspaceID kernel.WorkspaceID `json:"workspace_id"`
	PluginID    string             `json:"plugin_id"`
	Enabled     bool               `json:"enabled"`
	EnabledBy   kernel.MemberID    `json:"enabled_by"`
	EnabledAt   time.Time          `json:"enabled_at"`
}

func (p *PluginRecord) MarkPROpen(url string) error {
	switch p.Status {
	case PluginStatusUploaded, PluginStatusPRFailed, PluginStatusCIFailed:
		p.Status = PluginStatusPROpen
		p.PRURL = url
		p.touch()
		return nil
	default:
		return ErrInvalidPluginStatusTransition
	}
}

func (p *PluginRecord) MarkMerged() error {
	if p.Status != PluginStatusPROpen {
		return ErrInvalidPluginStatusTransition
	}
	p.Status = PluginStatusMerged
	p.touch()
	return nil
}

func (p *PluginRecord) MarkDeployed() error {
	if p.Status != PluginStatusMerged {
		return ErrInvalidPluginStatusTransition
	}
	p.Status = PluginStatusDeployed
	p.touch()
	return nil
}

func (p *PluginRecord) MarkPRFailed() error {
	switch p.Status {
	case PluginStatusUploaded, PluginStatusPROpen:
		p.Status = PluginStatusPRFailed
		p.touch()
		return nil
	default:
		return ErrInvalidPluginStatusTransition
	}
}

func (p *PluginRecord) MarkCIFailed() error {
	if p.Status != PluginStatusMerged {
		return ErrInvalidPluginStatusTransition
	}
	p.Status = PluginStatusCIFailed
	p.touch()
	return nil
}

func (p *PluginRecord) MarkUninstalling(prURL string) error {
	if p.Status != PluginStatusDeployed {
		return ErrInvalidPluginStatusTransition
	}
	p.Status = PluginStatusUninstalling
	p.PRURL = prURL
	p.touch()
	return nil
}

func (p *PluginRecord) MarkUninstallMerged() error {
	if p.Status != PluginStatusUninstalling {
		return ErrInvalidPluginStatusTransition
	}
	p.Status = PluginStatusUninstallMerged
	p.touch()
	return nil
}

func (p *PluginRecord) CanEnable() bool {
	return p.Status == PluginStatusDeployed
}

func (p *PluginRecord) touch() {
	p.UpdatedAt = time.Now().UTC()
}
