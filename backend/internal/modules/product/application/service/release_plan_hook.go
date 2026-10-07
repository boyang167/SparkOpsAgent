package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/product/domain/model"
)

type ReleasePlanHookNotifier struct {
	publicBase string
	client     *http.Client
}

func NewReleasePlanHookNotifier(publicBase string) *ReleasePlanHookNotifier {
	base := strings.TrimRight(strings.TrimSpace(publicBase), "/")
	return &ReleasePlanHookNotifier{
		publicBase: base,
		client:     &http.Client{Timeout: 8 * time.Second},
	}
}

func (n *ReleasePlanHookNotifier) Send(ctx context.Context, plan *model.ReleasePlan, event model.ReleasePlanHookEvent) error {
	if n == nil || plan == nil || plan.HookState == nil {
		return nil
	}
	hookURL := strings.TrimSpace(plan.HookSettings.EventURL(event))
	if hookURL == "" || !allowedWebhookURL(hookURL) {
		return nil
	}
	passURL := n.publicBase + "/api/v1/public/release-plans/hook/" + url.PathEscape(plan.HookState.Token) + "/pass"
	failURL := n.publicBase + "/api/v1/public/release-plans/hook/" + url.PathEscape(plan.HookState.Token) + "/fail"
	body, err := json.Marshal(map[string]any{
		"event":      event,
		"plan_id":    plan.ID.String(),
		"plan_name":  plan.Name,
		"product_id": plan.ProductID.String(),
		"status":     plan.Status,
		"pass_url":   passURL,
		"fail_url":   failURL,
		"token":      plan.HookState.Token,
	})
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, hookURL, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := n.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, resp.Body)
	if resp.StatusCode >= 300 {
		return fmt.Errorf("hook webhook status %d", resp.StatusCode)
	}
	return nil
}
