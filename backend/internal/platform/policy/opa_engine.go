package policy

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/sparkops/sparkops-agent/backend/internal/platform/config"
)

// OPAEngine delegates policy decisions to an OPA REST API (POST /v1/data/{path}).
type OPAEngine struct {
	baseURL    string
	dataPath   string
	httpClient *http.Client
	// in-memory mirror for Add/Remove when OPA is read-only fallback
	local *CasbinEngine
}

func NewOPAEngine(cfg config.OPAConfig) *OPAEngine {
	return &OPAEngine{
		baseURL:  cfg.URL,
		dataPath: cfg.DataPath,
		httpClient: &http.Client{
			Timeout: 5 * time.Second,
		},
	}
}

type opaInput struct {
	Input map[string]string `json:"input"`
}

type opaResult struct {
	Result bool `json:"result"`
}

func (e *OPAEngine) Enforce(ctx context.Context, subject, domain, object, action string) (bool, error) {
	if e.baseURL == "" {
		return false, nil
	}
	payload, _ := json.Marshal(opaInput{Input: map[string]string{
		"sub": subject, "dom": domain, "obj": object, "act": action,
	}})
	url := e.baseURL + "/v1/data/" + e.dataPath
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		return false, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := e.httpClient.Do(req)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return false, fmt.Errorf("opa returned %d", resp.StatusCode)
	}
	var out opaResult
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return false, err
	}
	return out.Result, nil
}

func (e *OPAEngine) AddPolicy(subject, domain, object, action string) error {
	if e.local != nil {
		return e.local.AddPolicy(subject, domain, object, action)
	}
	return nil
}

func (e *OPAEngine) RemovePolicy(subject, domain, object, action string) error {
	if e.local != nil {
		return e.local.RemovePolicy(subject, domain, object, action)
	}
	return nil
}

func (e *OPAEngine) RemoveFilteredPolicy(fieldIndex int, fieldValues ...string) error {
	if e.local != nil {
		return e.local.RemoveFilteredPolicy(fieldIndex, fieldValues...)
	}
	return nil
}

func (e *OPAEngine) AddGroupingPolicy(subject, role, domain string) error {
	if e.local != nil {
		return e.local.AddGroupingPolicy(subject, role, domain)
	}
	return nil
}

func (e *OPAEngine) RemoveGroupingPolicy(subject, role, domain string) error {
	if e.local != nil {
		return e.local.RemoveGroupingPolicy(subject, role, domain)
	}
	return nil
}

func (e *OPAEngine) RemoveFilteredGroupingPolicy(fieldIndex int, fieldValues ...string) error {
	if e.local != nil {
		return e.local.RemoveFilteredGroupingPolicy(fieldIndex, fieldValues...)
	}
	return nil
}

func (e *OPAEngine) SyncRoleAPIPolicies(roleID, workspaceID string, policies [][2]string) error {
	if e.local != nil {
		return e.local.SyncRoleAPIPolicies(roleID, workspaceID, policies)
	}
	return nil
}

func (e *OPAEngine) SavePolicy() error {
	if e.local != nil {
		return e.local.SavePolicy()
	}
	return nil
}

var _ Engine = (*OPAEngine)(nil)
