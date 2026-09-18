package engine

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

	"github.com/sparkops/sparkops-agent/backend/internal/modules/product_workflow/domain/model"
)

func (s *Scheduler) emitNotification(ctx context.Context, wf *model.ProductWorkflow, run *model.ProductWorkflowRun, event string) {
	if wf == nil && run != nil {
		loaded, err := s.repo.FindWorkflowByID(ctx, run.WorkflowID)
		if err != nil || loaded == nil {
			return
		}
		wf = loaded
	}
	if wf == nil {
		return
	}
	s.emitNotes(ctx, wf, run, wf.Notifications, event)
}

func (s *Scheduler) emitJobNotification(ctx context.Context, wf *model.ProductWorkflow, run *model.ProductWorkflowRun, spec map[string]any, event string) {
	s.emitNotes(ctx, wf, run, notificationsFromJobSpec(spec), event)
}

func (s *Scheduler) emitNotes(ctx context.Context, wf *model.ProductWorkflow, run *model.ProductWorkflowRun, notes []model.WorkflowNotification, event string) {
	if run == nil || strings.TrimSpace(event) == "" || !notificationMatches(notes, event) {
		return
	}
	copied := append([]model.WorkflowNotification(nil), notes...)
	if vars := VariablesFromRun(run); len(vars) > 0 {
		for i := range copied {
			copied[i].WebhookURL = Interpolate(copied[i].WebhookURL, vars)
			copied[i].Targets = Interpolate(copied[i].Targets, vars)
		}
	}
	name := ""
	if wf != nil {
		name = wf.Name
	}
	runID := run.ID.String()
	status := string(run.Status)
	message := run.ErrorMessage
	go func() {
		sendCtx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
		defer cancel()
		dispatchNotifications(sendCtx, copied, name, runID, status, message, event)
	}()
}

func notificationsFromJobSpec(spec map[string]any) []model.WorkflowNotification {
	if spec == nil {
		return nil
	}
	switch raw := spec["notify"].(type) {
	case []any:
		notes := make([]model.WorkflowNotification, 0, len(raw))
		for i, item := range raw {
			row, ok := item.(map[string]any)
			if !ok || notifyDisabled(row) {
				continue
			}
			notes = append(notes, notificationFromMap(row, fmt.Sprintf("job-notify-%d", i+1)))
		}
		return model.NormalizeNotifications(notes)
	case map[string]any:
		if notifyDisabled(raw) {
			return nil
		}
		return model.NormalizeNotifications([]model.WorkflowNotification{notificationFromMap(raw, "job-notify")})
	default:
		return nil
	}
}

func notifyDisabled(raw map[string]any) bool {
	enabled, ok := raw["enabled"].(bool)
	return ok && !enabled
}

func notificationFromMap(raw map[string]any, fallbackID string) model.WorkflowNotification {
	note := model.WorkflowNotification{
		ID:         specString(raw, "id"),
		Channel:    specString(raw, "channel"),
		WebhookURL: specString(raw, "webhook_url"),
		Targets:    specString(raw, "targets"),
	}
	if enabled, ok := raw["enabled"].(bool); ok {
		note.Enabled = &enabled
	}
	if note.ID == "" {
		note.ID = fallbackID
	}
	if events, ok := raw["events"].([]any); ok {
		for _, item := range events {
			if event, ok := item.(string); ok {
				note.Events = append(note.Events, event)
			}
		}
		return note
	}
	if raw["on_success"] == true {
		note.Events = append(note.Events, model.NotifyEventPassed)
	}
	if raw["on_failure"] != false {
		note.Events = append(note.Events, model.NotifyEventFailed)
	}
	return note
}

func specString(raw map[string]any, key string) string {
	value, _ := raw[key].(string)
	return strings.TrimSpace(value)
}

func notificationMatches(notes []model.WorkflowNotification, event string) bool {
	for _, note := range notes {
		if !note.Active() {
			continue
		}
		for _, item := range note.Events {
			if item == event {
				return true
			}
		}
	}
	return false
}

func dispatchNotifications(ctx context.Context, notes []model.WorkflowNotification, workflowName, runID, status, errMessage, event string) {
	client := &http.Client{Timeout: 6 * time.Second}
	text := notificationText(workflowName, runID, status, errMessage, event)
	for _, note := range notes {
		if !note.Active() || !containsEvent(note.Events, event) {
			continue
		}
		endpoint := strings.TrimSpace(note.WebhookURL)
		if !webhookChannel(note.Channel) || !allowedWebhook(endpoint) {
			continue
		}
		body, err := notificationBody(note.Channel, text, event, workflowName, runID, status)
		if err != nil {
			continue
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
		if err != nil {
			continue
		}
		req.Header.Set("Content-Type", "application/json")
		resp, err := client.Do(req)
		if err != nil {
			continue
		}
		_, _ = io.Copy(io.Discard, resp.Body)
		_ = resp.Body.Close()
	}
}

func containsEvent(events []string, event string) bool {
	for _, item := range events {
		if item == event {
			return true
		}
	}
	return false
}

func webhookChannel(channel string) bool {
	switch channel {
	case model.NotifyChannelFeishuBot, model.NotifyChannelFeishuApp, model.NotifyChannelDingTalk,
		model.NotifyChannelWeCom, model.NotifyChannelTeams, model.NotifyChannelWebhook:
		return true
	default:
		return false
	}
}

func allowedWebhook(raw string) bool {
	parsed, err := url.Parse(raw)
	if err != nil {
		return false
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return false
	}
	return parsed.Host != ""
}

func notificationText(workflowName, runID, status, errMessage, event string) string {
	label := model.NotifyEventLabel(event)
	text := fmt.Sprintf("工作流「%s」%s", strings.TrimSpace(workflowName), label)
	if runID != "" {
		text += fmt.Sprintf("，运行 %s", runID)
	}
	if status != "" {
		text += fmt.Sprintf("，状态 %s", status)
	}
	if strings.TrimSpace(errMessage) != "" {
		text += "：" + strings.TrimSpace(errMessage)
	}
	return text
}

func notificationBody(channel, text, event, workflowName, runID, status string) ([]byte, error) {
	switch channel {
	case model.NotifyChannelFeishuBot, model.NotifyChannelFeishuApp:
		return json.Marshal(map[string]any{
			"msg_type": "text",
			"content":  map[string]string{"text": text},
		})
	case model.NotifyChannelDingTalk, model.NotifyChannelWeCom:
		return json.Marshal(map[string]any{
			"msgtype": "text",
			"text":    map[string]string{"content": text},
		})
	case model.NotifyChannelTeams:
		return json.Marshal(map[string]string{"text": text})
	default:
		return json.Marshal(map[string]string{
			"event":    event,
			"workflow": workflowName,
			"run_id":   runID,
			"status":   status,
			"text":     text,
		})
	}
}
