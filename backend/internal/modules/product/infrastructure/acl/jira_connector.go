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
	"net/url"
	"strings"
	"time"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/product/domain"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/product/domain/port"
	wsmodel "github.com/sparkops/sparkops-agent/backend/internal/modules/workspace_app/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/crypto"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
	"gorm.io/gorm"
)

type JiraConnector struct {
	db     *gorm.DB
	box    *crypto.SecretBox
	client *http.Client
}

func NewJiraConnector(db *gorm.DB, box *crypto.SecretBox) *JiraConnector {
	return &JiraConnector{
		db:  db,
		box: box,
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

type jiraCredentials struct {
	baseURL  string
	username string
	token    string
}

func (c *JiraConnector) GetSprintIssues(ctx context.Context, wsID kernel.WorkspaceID, jiraSprintID string, issueTypes []string) ([]port.JiraIssue, error) {
	creds, err := c.loadCredentials(ctx, wsID)
	if err != nil {
		return nil, err
	}
	typeSet := map[string]struct{}{}
	for _, t := range issueTypes {
		typeSet[strings.ToLower(strings.TrimSpace(t))] = struct{}{}
	}
	var all []port.JiraIssue
	startAt := 0
	for {
		u := fmt.Sprintf("%s/rest/agile/1.0/sprint/%s/issue?startAt=%d&maxResults=50", creds.baseURL, url.PathEscape(jiraSprintID), startAt)
		body, err := c.doRequest(ctx, creds, http.MethodGet, u, nil)
		if err != nil {
			return nil, err
		}
		var page struct {
			Issues []json.RawMessage `json:"issues"`
			Total  int               `json:"total"`
		}
		if err := json.Unmarshal(body, &page); err != nil {
			return nil, err
		}
		for _, raw := range page.Issues {
			issue, err := parseJiraIssue(raw, creds.baseURL)
			if err != nil {
				continue
			}
			if len(typeSet) > 0 {
				if _, ok := typeSet[strings.ToLower(issue.IssueType)]; !ok {
					continue
				}
			}
			all = append(all, issue)
		}
		startAt += len(page.Issues)
		if startAt >= page.Total || len(page.Issues) == 0 {
			break
		}
	}
	return all, nil
}

func (c *JiraConnector) GetIssue(ctx context.Context, wsID kernel.WorkspaceID, issueKey string) (*port.JiraIssue, error) {
	creds, err := c.loadCredentials(ctx, wsID)
	if err != nil {
		return nil, err
	}
	u := fmt.Sprintf("%s/rest/api/2/issue/%s", creds.baseURL, url.PathEscape(strings.TrimSpace(issueKey)))
	body, err := c.doRequest(ctx, creds, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	issue, err := parseJiraIssue(body, creds.baseURL)
	if err != nil {
		return nil, err
	}
	return &issue, nil
}

func (c *JiraConnector) TransitionIssue(ctx context.Context, wsID kernel.WorkspaceID, issueKey, targetStatus string) error {
	creds, err := c.loadCredentials(ctx, wsID)
	if err != nil {
		return err
	}
	issueKey = strings.TrimSpace(issueKey)
	target := strings.ToLower(strings.TrimSpace(targetStatus))
	if target == "" {
		target = "done"
	}
	listURL := fmt.Sprintf("%s/rest/api/2/issue/%s/transitions", creds.baseURL, url.PathEscape(issueKey))
	body, err := c.doRequest(ctx, creds, http.MethodGet, listURL, nil)
	if err != nil {
		return err
	}
	var transitions struct {
		Transitions []struct {
			ID   string `json:"id"`
			Name string `json:"name"`
			To   struct {
				Name           string `json:"name"`
				StatusCategory struct {
					Key string `json:"key"`
				} `json:"statusCategory"`
			} `json:"to"`
		} `json:"transitions"`
	}
	if err := json.Unmarshal(body, &transitions); err != nil {
		return err
	}
	var transitionID string
	for _, tr := range transitions.Transitions {
		name := strings.ToLower(tr.Name)
		toName := strings.ToLower(tr.To.Name)
		category := strings.ToLower(tr.To.StatusCategory.Key)
		if strings.Contains(name, target) || strings.Contains(toName, target) || category == target || category == "done" && (target == "done" || target == "complete") {
			transitionID = tr.ID
			break
		}
	}
	if transitionID == "" {
		return fmt.Errorf("no jira transition found for target %q", targetStatus)
	}
	postURL := listURL
	payload, _ := json.Marshal(map[string]any{"transition": map[string]string{"id": transitionID}})
	_, err = c.doRequest(ctx, creds, http.MethodPost, postURL, payload)
	return err
}

func (c *JiraConnector) loadCredentials(ctx context.Context, wsID kernel.WorkspaceID) (jiraCredentials, error) {
	var row struct {
		ServiceURL     string
		Username       string
		CredentialBlob string
	}
	err := c.db.WithContext(ctx).Table("workspace_app.secrets").
		Where("workspace_id = ? AND LOWER(service_type) = ? AND status = ?", wsID.String(), "jira", string(wsmodel.SecretStatusActive)).
		Order("is_default DESC, created_at ASC").
		Limit(1).
		First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return jiraCredentials{}, domain.ErrJiraNotConfigured
	}
	if err != nil {
		return jiraCredentials{}, err
	}
	baseURL := strings.TrimRight(strings.TrimSpace(row.ServiceURL), "/")
	if baseURL == "" {
		return jiraCredentials{}, domain.ErrJiraNotConfigured
	}
	plain, err := c.box.Decrypt(row.CredentialBlob)
	if err != nil {
		return jiraCredentials{}, domain.ErrJiraNotConfigured
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
		return jiraCredentials{}, domain.ErrJiraNotConfigured
	}
	return jiraCredentials{
		baseURL:  baseURL,
		username: strings.TrimSpace(row.Username),
		token:    token,
	}, nil
}

func (c *JiraConnector) doRequest(ctx context.Context, creds jiraCredentials, method, reqURL string, body []byte) ([]byte, error) {
	var reader io.Reader
	if len(body) > 0 {
		reader = bytes.NewReader(body)
	}
	req, err := http.NewRequestWithContext(ctx, method, reqURL, reader)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Basic "+base64.StdEncoding.EncodeToString([]byte(creds.username+":"+creds.token)))
	req.Header.Set("Accept", "application/json")
	if len(body) > 0 {
		req.Header.Set("Content-Type", "application/json")
	}
	res, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	respBody, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}
	if res.StatusCode >= 400 {
		return nil, fmt.Errorf("jira api %s %s: %s", method, reqURL, strings.TrimSpace(string(respBody)))
	}
	return respBody, nil
}

func parseJiraIssue(raw []byte, baseURL string) (port.JiraIssue, error) {
	var payload struct {
		Key    string `json:"key"`
		ID     string `json:"id"`
		Fields struct {
			Summary     string `json:"summary"`
			Description any    `json:"description"`
			IssueType   struct {
				Name string `json:"name"`
			} `json:"issuetype"`
			Status struct {
				Name string `json:"name"`
			} `json:"status"`
			Priority struct {
				Name string `json:"name"`
			} `json:"priority"`
		} `json:"fields"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return port.JiraIssue{}, err
	}
	desc := ""
	switch v := payload.Fields.Description.(type) {
	case string:
		desc = v
	case map[string]any:
		desc = extractADFText(v)
	}
	issueURL := ""
	if payload.Key != "" {
		issueURL = baseURL + "/browse/" + payload.Key
	}
	return port.JiraIssue{
		Key:         payload.Key,
		ID:          payload.ID,
		Summary:     payload.Fields.Summary,
		Description: desc,
		IssueType:   payload.Fields.IssueType.Name,
		Status:      payload.Fields.Status.Name,
		Priority:    payload.Fields.Priority.Name,
		URL:         issueURL,
	}, nil
}

func extractADFText(node map[string]any) string {
	var parts []string
	if text, ok := node["text"].(string); ok {
		parts = append(parts, text)
	}
	if content, ok := node["content"].([]any); ok {
		for _, child := range content {
			if m, ok := child.(map[string]any); ok {
				parts = append(parts, extractADFText(m))
			}
		}
	}
	return strings.Join(parts, " ")
}
