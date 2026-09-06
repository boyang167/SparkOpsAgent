package jenkins

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/delivery/domain"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/delivery/domain/port"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/crypto"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
	"gorm.io/gorm"
)

type StaticProvider struct {
	executor port.JenkinsExecutor
}

func NewStaticProvider(executor port.JenkinsExecutor) *StaticProvider {
	return &StaticProvider{executor: executor}
}

func (p *StaticProvider) ForProject(context.Context, kernel.WorkspaceID, kernel.ProjectID) (port.JenkinsExecutor, error) {
	return p.executor, nil
}

type ProjectExecutorProvider struct {
	db        *gorm.DB
	box       *crypto.SecretBox
	jobSuffix string
}

func NewProjectExecutorProvider(db *gorm.DB, box *crypto.SecretBox, jobSuffix string) *ProjectExecutorProvider {
	return &ProjectExecutorProvider{db: db, box: box, jobSuffix: jobSuffix}
}

func (p *ProjectExecutorProvider) ForProject(
	ctx context.Context,
	wsID kernel.WorkspaceID,
	projectID kernel.ProjectID,
) (port.JenkinsExecutor, error) {
	conn, err := p.loadProjectJenkinsConnection(ctx, wsID, projectID)
	if err != nil {
		return nil, err
	}
	return NewExecutorFromConnection(conn, p.jobSuffix)
}

func (p *ProjectExecutorProvider) loadProjectJenkinsConnection(
	ctx context.Context,
	wsID kernel.WorkspaceID,
	projectID kernel.ProjectID,
) (ConnectionConfig, error) {
	var row struct {
		JenkinsSecretID *string
	}
	err := p.db.WithContext(ctx).Table("workspace_app.projects").
		Select("jenkins_secret_id").
		Where("id = ? AND workspace_id = ?", projectID.String(), wsID.String()).
		First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) || row.JenkinsSecretID == nil || strings.TrimSpace(*row.JenkinsSecretID) == "" {
		return ConnectionConfig{}, domain.ErrProjectJenkinsNotConfigured
	}
	if err != nil {
		return ConnectionConfig{}, err
	}
	return p.loadSecretConnection(ctx, wsID, strings.TrimSpace(*row.JenkinsSecretID))
}

func (p *ProjectExecutorProvider) loadSecretConnection(
	ctx context.Context,
	wsID kernel.WorkspaceID,
	secretID string,
) (ConnectionConfig, error) {
	var row struct {
		ServiceType    string
		ServiceURL     string
		Username       string
		CredentialBlob string
		Status         string
	}
	err := p.db.WithContext(ctx).Table("workspace_app.secrets").
		Where("id = ? AND workspace_id = ? AND status = ?", secretID, wsID.String(), "active").
		First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return ConnectionConfig{}, domain.ErrProjectJenkinsNotConfigured
	}
	if err != nil {
		return ConnectionConfig{}, err
	}
	if row.ServiceType != "Jenkins" {
		return ConnectionConfig{}, domain.ErrProjectJenkinsNotConfigured
	}
	plain, err := p.box.Decrypt(row.CredentialBlob)
	if err != nil {
		return ConnectionConfig{}, fmt.Errorf("decrypt jenkins secret: %w", err)
	}
	var credentials struct {
		Password string `json:"password"`
		Token    string `json:"token"`
		APIKey   string `json:"api_key"`
	}
	if plain != "" {
		if err := json.Unmarshal([]byte(plain), &credentials); err != nil {
			return ConnectionConfig{}, fmt.Errorf("parse jenkins secret: %w", err)
		}
	}
	conn := ConnectionConfig{
		URL:      strings.TrimSpace(row.ServiceURL),
		User:     strings.TrimSpace(row.Username),
		Password: strings.TrimSpace(credentials.Password),
		Token:    resolveToken(credentials.Token, credentials.APIKey, credentials.Password),
	}
	if conn.URL == "" {
		return ConnectionConfig{}, domain.ErrProjectJenkinsNotConfigured
	}
	if conn.Token == "" {
		return ConnectionConfig{}, domain.ErrProjectJenkinsNotConfigured
	}
	return conn, nil
}

var (
	_ port.JenkinsExecutorProvider = (*StaticProvider)(nil)
	_ port.JenkinsExecutorProvider = (*ProjectExecutorProvider)(nil)
)
