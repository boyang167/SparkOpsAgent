package service

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/product/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/httperror"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/messaging"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

type ReleasePlanHookNotifier struct {
	publicBase string
	client     *http.Client
}

type ReleasePlanHookPlanLoader interface {
	FindPlanByID(context.Context, kernel.ReleasePlanID) (*model.ReleasePlan, error)
}

type ReleasePlanHookSender interface {
	SendWithIdempotency(context.Context, *model.ReleasePlan, model.ReleasePlanHookEvent, string) error
}

func SendReleasePlanHookOperation(
	ctx context.Context,
	plans ReleasePlanHookPlanLoader,
	sender ReleasePlanHookSender,
	planID kernel.ReleasePlanID,
	event model.ReleasePlanHookEvent,
	tokenDigest string,
	idempotencyKey string,
) error {
	if plans == nil || sender == nil {
		return messaging.Permanent(fmt.Errorf("release plan hook operation dependencies are required"))
	}
	plan, err := plans.FindPlanByID(ctx, planID)
	if err != nil {
		return err
	}
	state := plan.HookState
	if state == nil || state.Status != "pending" || state.Event != event ||
		ReleasePlanHookTokenDigest(state.Token) != tokenDigest {
		return nil
	}
	return sender.SendWithIdempotency(ctx, plan, event, idempotencyKey)
}

func ReleasePlanHookTokenDigest(token string) string {
	sum := sha256.Sum256([]byte(token))
	return fmt.Sprintf("%x", sum[:])
}

func NewReleasePlanHookNotifier(publicBase string) *ReleasePlanHookNotifier {
	base := strings.TrimRight(strings.TrimSpace(publicBase), "/")
	return &ReleasePlanHookNotifier{
		publicBase: base,
		client:     &http.Client{Timeout: 8 * time.Second},
	}
}

func (n *ReleasePlanHookNotifier) Send(ctx context.Context, plan *model.ReleasePlan, event model.ReleasePlanHookEvent) error {
	return n.SendWithIdempotency(ctx, plan, event, "")
}

func (n *ReleasePlanHookNotifier) SendWithIdempotency(ctx context.Context, plan *model.ReleasePlan, event model.ReleasePlanHookEvent, idempotencyKey string) error {
	if n == nil {
		return messaging.Permanent(fmt.Errorf("release plan hook notifier is required"))
	}
	if err := ValidateReleasePlanHook(plan, event); err != nil {
		return messaging.Permanent(err)
	}
	if !releasePlanHookConfigured(plan, event) {
		return nil
	}
	if plan.HookState == nil || strings.TrimSpace(plan.HookState.Token) == "" {
		return messaging.Permanent(fmt.Errorf("release plan hook token is required"))
	}
	hookURL := strings.TrimSpace(plan.HookSettings.EventURL(event))
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
		return messaging.Permanent(httperror.SanitizeNetwork(err))
	}
	req.Header.Set("Content-Type", "application/json")
	if idempotencyKey != "" {
		req.Header.Set("Idempotency-Key", idempotencyKey)
	}
	resp, err := n.client.Do(req)
	if err != nil {
		return httperror.SanitizeNetwork(err)
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, resp.Body)
	if resp.StatusCode >= 300 {
		err := fmt.Errorf("hook webhook status %d", resp.StatusCode)
		if resp.StatusCode >= 400 && resp.StatusCode < 500 && resp.StatusCode != http.StatusRequestTimeout && resp.StatusCode != http.StatusTooManyRequests {
			return messaging.Permanent(err)
		}
		return err
	}
	return nil
}

func ValidateReleasePlanHook(plan *model.ReleasePlan, event model.ReleasePlanHookEvent) error {
	if plan == nil {
		return fmt.Errorf("release plan is required")
	}
	if !releasePlanHookConfigured(plan, event) {
		return nil
	}
	if !allowedWebhookURL(strings.TrimSpace(plan.HookSettings.EventURL(event))) {
		return fmt.Errorf("release plan hook URL is invalid for event %s", event)
	}
	return nil
}

func releasePlanHookConfigured(plan *model.ReleasePlan, event model.ReleasePlanHookEvent) bool {
	if plan == nil || !plan.HookSettings.Enabled {
		return false
	}
	cfg, ok := plan.HookSettings.Events[event]
	return ok && cfg.Enabled
}
