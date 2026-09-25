package acl

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/product/domain"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/product/domain/port"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/crypto"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
	"gorm.io/gorm"
)

type TapdConnector struct {
	base *pmConnectorBase
}

func NewTapdConnector(db *gorm.DB, box *crypto.SecretBox) *TapdConnector {
	return &TapdConnector{base: newPmConnectorBase(db, box, "tapd", "https://api.tapd.cn")}
}

func (c *TapdConnector) Provider() string { return port.PmProviderTapd }

func (c *TapdConnector) GetIterationIssues(ctx context.Context, wsID kernel.WorkspaceID, pmWorkspaceID, iterationID string, issueTypes []string) ([]port.PmIssue, error) {
	pmWorkspaceID = strings.TrimSpace(pmWorkspaceID)
	iterationID = strings.TrimSpace(iterationID)
	if pmWorkspaceID == "" || iterationID == "" {
		return nil, domain.ErrPmInvalidRequest
	}
	typeSet := map[string]struct{}{}
	for _, t := range issueTypes {
		typeSet[strings.ToLower(strings.TrimSpace(t))] = struct{}{}
	}
	query := url.Values{}
	query.Set("workspace_id", pmWorkspaceID)
	query.Set("iteration_id", iterationID)
	query.Set("limit", "200")
	body, err := c.base.doRequest(ctx, wsID, "GET", "/stories?"+query.Encode(), nil, "")
	if err != nil {
		return nil, err
	}
	return parseTapdStories(body, pmWorkspaceID, typeSet)
}

func (c *TapdConnector) GetIssue(ctx context.Context, wsID kernel.WorkspaceID, pmWorkspaceID, issueKey string) (*port.PmIssue, error) {
	pmWorkspaceID = strings.TrimSpace(pmWorkspaceID)
	issueKey = strings.TrimSpace(issueKey)
	if pmWorkspaceID == "" || issueKey == "" {
		return nil, domain.ErrPmInvalidRequest
	}
	query := url.Values{}
	query.Set("workspace_id", pmWorkspaceID)
	query.Set("id", issueKey)
	body, err := c.base.doRequest(ctx, wsID, "GET", "/stories?"+query.Encode(), nil, "")
	if err != nil {
		return nil, err
	}
	items, err := parseTapdStories(body, pmWorkspaceID, nil)
	if err != nil {
		return nil, err
	}
	if len(items) == 0 {
		return &port.PmIssue{Key: issueKey, ID: issueKey}, nil
	}
	return &items[0], nil
}

func (c *TapdConnector) TransitionIssue(ctx context.Context, wsID kernel.WorkspaceID, pmWorkspaceID, issueKey, targetStatus string) error {
	pmWorkspaceID = strings.TrimSpace(pmWorkspaceID)
	form := url.Values{}
	form.Set("workspace_id", pmWorkspaceID)
	form.Set("id", strings.TrimSpace(issueKey))
	form.Set("status", strings.TrimSpace(targetStatus))
	_, err := c.base.doRequest(ctx, wsID, "POST", "/story/update", []byte(form.Encode()), "application/x-www-form-urlencoded")
	return err
}

func parseTapdStories(body []byte, pmWorkspaceID string, typeSet map[string]struct{}) ([]port.PmIssue, error) {
	var payload struct {
		Data []struct {
			Story struct {
				ID       string `json:"id"`
				Name     string `json:"name"`
				Status   string `json:"status"`
				Priority string `json:"priority"`
				Type     string `json:"type"`
			} `json:"Story"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, err
	}
	out := make([]port.PmIssue, 0, len(payload.Data))
	for _, row := range payload.Data {
		st := row.Story
		if st.ID == "" {
			continue
		}
		if len(typeSet) > 0 {
			if _, ok := typeSet[strings.ToLower(st.Type)]; !ok {
				continue
			}
		}
		out = append(out, port.PmIssue{
			Key: st.ID, ID: st.ID, Summary: st.Name, IssueType: st.Type,
			Status: st.Status, Priority: st.Priority,
			URL: fmt.Sprintf("https://www.tapd.cn/%s/prong/stories/view/%s", pmWorkspaceID, st.ID),
		})
	}
	return out, nil
}
