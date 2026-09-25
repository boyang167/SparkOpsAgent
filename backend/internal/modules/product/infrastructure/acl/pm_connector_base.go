package acl

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/product/domain"
	wsmodel "github.com/sparkops/sparkops-agent/backend/internal/modules/workspace_app/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/crypto"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
	"gorm.io/gorm"
)

type pmCredentials struct {
	baseURL  string
	username string
	token    string
}

type pmConnectorBase struct {
	db          *gorm.DB
	box         *crypto.SecretBox
	client      *http.Client
	serviceType string
	defaultBase string
}

func newPmConnectorBase(db *gorm.DB, box *crypto.SecretBox, serviceType, defaultBase string) *pmConnectorBase {
	return &pmConnectorBase{
		db: db, box: box, serviceType: serviceType, defaultBase: defaultBase,
		client: &http.Client{Timeout: 30 * time.Second},
	}
}

func (b *pmConnectorBase) loadCredentials(ctx context.Context, wsID kernel.WorkspaceID) (pmCredentials, error) {
	var row struct {
		ServiceURL     string
		Username       string
		CredentialBlob string
	}
	err := b.db.WithContext(ctx).Table("workspace_app.secrets").
		Where("workspace_id = ? AND LOWER(service_type) = ? AND status = ?", wsID.String(), strings.ToLower(b.serviceType), string(wsmodel.SecretStatusActive)).
		Order("is_default DESC, created_at ASC").
		Limit(1).
		First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return pmCredentials{}, domain.ErrPmNotConfigured
	}
	if err != nil {
		return pmCredentials{}, err
	}
	baseURL := strings.TrimRight(strings.TrimSpace(row.ServiceURL), "/")
	if baseURL == "" {
		baseURL = strings.TrimRight(b.defaultBase, "/")
	}
	plain, err := b.box.Decrypt(row.CredentialBlob)
	if err != nil {
		return pmCredentials{}, domain.ErrPmNotConfigured
	}
	var creds wsmodel.SecretCredentials
	if plain != "" {
		_ = json.Unmarshal([]byte(plain), &creds)
	}
	token := strings.TrimSpace(creds.Token)
	if token == "" {
		token = strings.TrimSpace(creds.APIKey)
	}
	if token == "" {
		token = strings.TrimSpace(creds.Password)
	}
	if token == "" || strings.TrimSpace(row.Username) == "" {
		return pmCredentials{}, domain.ErrPmNotConfigured
	}
	return pmCredentials{baseURL: baseURL, username: strings.TrimSpace(row.Username), token: token}, nil
}

func (b *pmConnectorBase) doRequest(ctx context.Context, wsID kernel.WorkspaceID, method, path string, body []byte, contentType string) ([]byte, error) {
	creds, err := b.loadCredentials(ctx, wsID)
	if err != nil {
		return nil, err
	}
	var reader io.Reader
	if len(body) > 0 {
		reader = bytes.NewReader(body)
	}
	reqURL := creds.baseURL + path
	req, err := http.NewRequestWithContext(ctx, method, reqURL, reader)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Basic "+base64.StdEncoding.EncodeToString([]byte(creds.username+":"+creds.token)))
	req.Header.Set("Accept", "application/json")
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	res, err := b.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	respBody, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}
	if res.StatusCode >= 400 {
		return nil, fmt.Errorf("%s api %s %s: %s", b.serviceType, method, reqURL, strings.TrimSpace(string(respBody)))
	}
	return respBody, nil
}
