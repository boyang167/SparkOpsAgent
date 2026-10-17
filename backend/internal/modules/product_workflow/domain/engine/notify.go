package engine

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/smtp"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/product_workflow/domain/model"
	workflowoutbound "github.com/sparkops/sparkops-agent/backend/internal/modules/product_workflow/infrastructure/outbound"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/messaging"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/events"
)

type ChannelConfig struct {
	SMTPHost     string
	SMTPPort     int
	SMTPUser     string
	SMTPPassword string
	SMTPFrom     string
	FeishuAppID  string
	FeishuSecret string
}

var (
	channelMu sync.Mutex
	channels  ChannelConfig
)

func ConfigureChannels(cfg ChannelConfig) {
	channelMu.Lock()
	channels = cfg
	channelMu.Unlock()
}

func currentChannels() ChannelConfig {
	channelMu.Lock()
	defer channelMu.Unlock()
	return channels
}

func (s *Scheduler) emitNotification(ctx context.Context, wf *model.ProductWorkflow, run *model.ProductWorkflowRun, event string) error {
	if wf == nil && run != nil {
		loaded, err := s.repo.FindWorkflowByID(ctx, run.WorkflowID)
		if err != nil || loaded == nil {
			return err
		}
		wf = loaded
	}
	if wf == nil {
		return nil
	}
	return s.emitNotes(ctx, wf, run, wf.Notifications, event)
}

func (s *Scheduler) emitJobNotification(ctx context.Context, wf *model.ProductWorkflow, run *model.ProductWorkflowRun, spec map[string]any, event string) error {
	return s.emitNotes(ctx, wf, run, notificationsFromJobSpec(spec), event)
}

func (s *Scheduler) emitNotes(ctx context.Context, wf *model.ProductWorkflow, run *model.ProductWorkflowRun, notes []model.WorkflowNotification, event string) error {
	if run == nil || strings.TrimSpace(event) == "" || !notificationMatches(notes, event) {
		return nil
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
	if s.operations != nil {
		metadata, ok := events.MetadataFromContext(ctx)
		if ok {
			for _, note := range copied {
				if !note.Active() || !containsEvent(note.Events, event) {
					continue
				}
				operationType := workflowoutbound.OperationTypeWebhookSend
				switch note.Channel {
				case model.NotifyChannelMail:
					operationType = workflowoutbound.OperationTypeMailSend
				case model.NotifyChannelFeishuUser:
					operationType = workflowoutbound.OperationTypeFeishuUserSend
				}
				body, err := notificationBody(note.Channel, notificationText(name, runID, status, message, event), event, name, runID, status)
				if err != nil {
					return err
				}
				if err := s.operations.Schedule(ctx, messaging.OperationRequest{
					Owner: metadata.Subscriber, OperationType: operationType,
					IdempotencyKey: metadata.EventID + ":" + metadata.Subscriber + ":" + operationType + ":" + note.ID,
					Payload: workflowoutbound.NotificationPayload{
						EventID: metadata.EventID, Target: note.ID, Channel: note.Channel,
						URL: note.WebhookURL, Targets: note.Targets, WorkflowName: name,
						RunID: runID, Status: status, ErrorMessage: message, Event: event,
						Body: body, Timeout: 8 * time.Second,
					},
				}); err != nil {
					return err
				}
			}
			return nil
		}
	}
	sendCtx, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()
	return dispatchNotifications(sendCtx, copied, name, runID, status, message, event)
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

func dispatchNotifications(ctx context.Context, notes []model.WorkflowNotification, workflowName, runID, status, errMessage, event string) error {
	client := &http.Client{Timeout: 6 * time.Second}
	text := notificationText(workflowName, runID, status, errMessage, event)
	for _, note := range notes {
		if !note.Active() || !containsEvent(note.Events, event) {
			continue
		}
		endpoint := strings.TrimSpace(note.WebhookURL)
		switch note.Channel {
		case model.NotifyChannelMail:
			if err := sendMail(currentChannels(), note.Targets, workflowName, text, ""); err != nil {
				return err
			}
			continue
		case model.NotifyChannelFeishuUser:
			if err := sendFeishuUsers(ctx, currentChannels(), note.Targets, text, ""); err != nil {
				return err
			}
			continue
		}
		if !webhookChannel(note.Channel) || !allowedWebhook(endpoint) {
			continue
		}
		body, err := notificationBody(note.Channel, text, event, workflowName, runID, status)
		if err != nil {
			return err
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
		if err != nil {
			return err
		}
		req.Header.Set("Content-Type", "application/json")
		resp, err := client.Do(req)
		if err != nil {
			return err
		}
		_, _ = io.Copy(io.Discard, resp.Body)
		_ = resp.Body.Close()
		if err := classifyNotificationHTTP(resp.StatusCode); err != nil {
			return err
		}
	}
	return nil
}

func classifyNotificationHTTP(status int) error {
	if status < http.StatusMultipleChoices {
		return nil
	}
	err := fmt.Errorf("notification endpoint returned status %d", status)
	if status == http.StatusRequestTimeout || status == http.StatusTooManyRequests || status >= http.StatusInternalServerError {
		return err
	}
	return messaging.Permanent(err)
}

func SendMailOperation(_ context.Context, payload workflowoutbound.NotificationPayload) error {
	return sendMail(currentChannels(), payload.Targets, payload.WorkflowName,
		notificationText(payload.WorkflowName, payload.RunID, payload.Status, payload.ErrorMessage, payload.Event),
		payload.IdempotencyKey)
}

func SendFeishuUserOperation(ctx context.Context, payload workflowoutbound.NotificationPayload) error {
	return sendFeishuUsers(ctx, currentChannels(), payload.Targets,
		notificationText(payload.WorkflowName, payload.RunID, payload.Status, payload.ErrorMessage, payload.Event),
		payload.IdempotencyKey)
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

func splitTargets(raw string) []string {
	fields := strings.FieldsFunc(raw, func(r rune) bool {
		return r == ',' || r == ';' || r == '\n' || r == ' ' || r == '\t'
	})
	out := make([]string, 0, len(fields))
	for _, field := range fields {
		field = strings.TrimSpace(field)
		if field != "" {
			out = append(out, field)
		}
	}
	return out
}

func sendMail(cfg ChannelConfig, targets, subject, body, idempotencyKey string) error {
	host := strings.TrimSpace(cfg.SMTPHost)
	recipients := splitTargets(targets)
	if host == "" || len(recipients) == 0 {
		return nil
	}
	port := cfg.SMTPPort
	if port == 0 {
		port = 587
	}
	from := strings.TrimSpace(cfg.SMTPFrom)
	if from == "" {
		from = cfg.SMTPUser
	}
	messageID := ""
	if idempotencyKey != "" {
		messageID = "Message-ID: <" + url.QueryEscape(idempotencyKey) + "@sparkops>\r\n"
	}
	msg := []byte("From: " + from + "\r\nTo: " + strings.Join(recipients, ",") + "\r\nSubject: " + subject + "\r\n" + messageID + "\r\n" + body)
	addr := fmt.Sprintf("%s:%d", host, port)
	var auth smtp.Auth
	if cfg.SMTPUser != "" {
		auth = smtp.PlainAuth("", cfg.SMTPUser, cfg.SMTPPassword, host)
	}
	return smtp.SendMail(addr, auth, from, recipients, msg)
}

func sendFeishuUsers(ctx context.Context, cfg ChannelConfig, targets, text, idempotencyKey string) error {
	if strings.TrimSpace(cfg.FeishuAppID) == "" || strings.TrimSpace(cfg.FeishuSecret) == "" {
		return nil
	}
	recipients := splitTargets(targets)
	if len(recipients) == 0 {
		return nil
	}
	token, err := feishuTenantToken(ctx, cfg.FeishuAppID, cfg.FeishuSecret)
	if err != nil {
		return err
	}
	client := &http.Client{Timeout: 8 * time.Second}
	for _, target := range recipients {
		idType := "open_id"
		if strings.Contains(target, "@") {
			idType = "email"
		}
		content, _ := json.Marshal(map[string]string{"text": text})
		raw, _ := json.Marshal(map[string]string{
			"receive_id": target, "msg_type": "text", "content": string(content),
		})
		endpoint := "https://open.feishu.cn/open-apis/im/v1/messages?receive_id_type=" + idType
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(raw))
		if err != nil {
			return err
		}
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("Content-Type", "application/json")
		if idempotencyKey != "" {
			req.Header.Set("X-SparkOps-Idempotency-Key", idempotencyKey)
		}
		resp, err := client.Do(req)
		if err != nil {
			return err
		}
		_, _ = io.Copy(io.Discard, resp.Body)
		_ = resp.Body.Close()
		if err := classifyNotificationHTTP(resp.StatusCode); err != nil {
			return err
		}
	}
	return nil
}

func feishuTenantToken(ctx context.Context, appID, secret string) (string, error) {
	raw, _ := json.Marshal(map[string]string{"app_id": appID, "app_secret": secret})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://open.feishu.cn/open-apis/auth/v3/tenant_access_token/internal", bytes.NewReader(raw))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := (&http.Client{Timeout: 8 * time.Second}).Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	var payload struct {
		TenantAccessToken string `json:"tenant_access_token"`
		Code              int    `json:"code"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return "", err
	}
	if payload.TenantAccessToken == "" {
		return "", fmt.Errorf("feishu token code %d", payload.Code)
	}
	return payload.TenantAccessToken, nil
}
