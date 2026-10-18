package engine

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/smtp"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/product_workflow/domain"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/product_workflow/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/product_workflow/domain/repository"
	workflowoutbound "github.com/sparkops/sparkops-agent/backend/internal/modules/product_workflow/infrastructure/outbound"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/httperror"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/messaging"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/events"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
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

type WorkflowWebhookResolver struct {
	repo repository.ProductWorkflowRepository
}

func NewWorkflowWebhookResolver(repo repository.ProductWorkflowRepository) *WorkflowWebhookResolver {
	return &WorkflowWebhookResolver{repo: repo}
}

func (r *WorkflowWebhookResolver) ResolveWebhook(
	ctx context.Context,
	payload workflowoutbound.NotificationPayload,
) (string, json.RawMessage, bool, error) {
	if r == nil || r.repo == nil {
		return "", nil, false, messaging.Permanent(fmt.Errorf("workflow notification repository is required"))
	}
	runID, err := kernel.ParseProductWorkflowRunID(payload.RunID)
	if err != nil {
		return "", nil, false, messaging.Permanent(err)
	}
	run, err := r.repo.FindRunByID(ctx, runID)
	if err != nil {
		if errors.Is(err, domain.ErrProductWorkflowRunNotFound) {
			return "", nil, false, nil
		}
		return "", nil, false, err
	}
	workflowID := run.WorkflowID
	if payload.WorkflowID != "" {
		workflowID, err = kernel.ParseProductWorkflowID(payload.WorkflowID)
		if err != nil {
			return "", nil, false, messaging.Permanent(err)
		}
	}
	wf, err := r.repo.FindWorkflowByID(ctx, workflowID)
	if err != nil {
		if errors.Is(err, domain.ErrProductWorkflowNotFound) {
			return "", nil, false, nil
		}
		return "", nil, false, err
	}
	notes := wf.Notifications
	if payload.Scope == "job" {
		job, ok := jobByName(wf, payload.JobID)
		if !ok {
			return "", nil, false, nil
		}
		notes = notificationsFromJobSpec(job.Spec)
	}
	var note *model.WorkflowNotification
	for i := range notes {
		if notes[i].ID == payload.NoteID {
			note = &notes[i]
			break
		}
	}
	if note == nil || !note.Active() || !containsEvent(note.Events, payload.Event) {
		return "", nil, false, nil
	}
	endpoint := Interpolate(note.WebhookURL, VariablesFromRun(run))
	if !allowedWebhook(endpoint) {
		return "", nil, false, messaging.Permanent(fmt.Errorf("workflow webhook configuration is invalid"))
	}
	body, err := notificationBody(
		note.Channel,
		notificationText(wf.Name, payload.RunID, payload.Status, payload.ErrorMessage, payload.Event),
		payload.Event, wf.Name, payload.RunID, payload.Status,
	)
	if err != nil {
		return "", nil, false, messaging.Permanent(err)
	}
	return endpoint, body, true, nil
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
	return s.emitNotes(ctx, wf, run, wf.Notifications, event, "workflow", "")
}

func (s *Scheduler) emitJobNotification(ctx context.Context, wf *model.ProductWorkflow, run *model.ProductWorkflowRun, jobID string, spec map[string]any, event string) error {
	return s.emitNotes(ctx, wf, run, notificationsFromJobSpec(spec), event, "job", jobID)
}

func (s *Scheduler) emitNotes(
	ctx context.Context,
	wf *model.ProductWorkflow,
	run *model.ProductWorkflowRun,
	notes []model.WorkflowNotification,
	event, scope, jobID string,
) error {
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
				target := strings.Join([]string{runID, scope, emptyTarget(jobID), event, emptyTarget(status), note.ID}, "/")
				if err := s.operations.Schedule(ctx, messaging.OperationRequest{
					Owner: metadata.Subscriber, OperationType: operationType,
					IdempotencyKey: metadata.EventID + ":" + metadata.Subscriber + ":" + operationType + ":" + target,
					Payload: workflowoutbound.NotificationPayload{
						EventID: metadata.EventID, Target: target, Channel: note.Channel,
						Targets: note.Targets, WorkflowID: wf.ID.String(), WorkflowName: name,
						RunID: runID, Scope: scope, JobID: jobID, NoteID: note.ID,
						Status: status, ErrorMessage: message, Event: event, Timeout: 8 * time.Second,
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

func emptyTarget(value string) string {
	if strings.TrimSpace(value) == "" {
		return "-"
	}
	return strings.TrimSpace(value)
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
			return httperror.SanitizeNetwork(err)
		}
		req.Header.Set("Content-Type", "application/json")
		resp, err := client.Do(req)
		if err != nil {
			return httperror.SanitizeNetwork(err)
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
	cfg := currentChannels()
	if strings.TrimSpace(cfg.SMTPHost) == "" {
		return messaging.Permanent(fmt.Errorf("SMTP host is required"))
	}
	if len(splitTargets(payload.Targets)) == 0 {
		return messaging.Permanent(fmt.Errorf("SMTP recipients are required"))
	}
	return sendMail(cfg, payload.Targets, payload.WorkflowName,
		notificationText(payload.WorkflowName, payload.RunID, payload.Status, payload.ErrorMessage, payload.Event),
		payload.IdempotencyKey)
}

func SendFeishuUserOperation(ctx context.Context, payload workflowoutbound.NotificationPayload) error {
	cfg := currentChannels()
	if strings.TrimSpace(cfg.FeishuAppID) == "" || strings.TrimSpace(cfg.FeishuSecret) == "" {
		return messaging.Permanent(fmt.Errorf("Feishu app credentials are required"))
	}
	if len(splitTargets(payload.Targets)) == 0 {
		return messaging.Permanent(fmt.Errorf("Feishu recipients are required"))
	}
	return sendFeishuUsers(ctx, cfg, payload.Targets,
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
	msg := buildSMTPMessage(from, recipients, subject, body, idempotencyKey)
	addr := fmt.Sprintf("%s:%d", host, port)
	var auth smtp.Auth
	if cfg.SMTPUser != "" {
		auth = smtp.PlainAuth("", cfg.SMTPUser, cfg.SMTPPassword, host)
	}
	return smtp.SendMail(addr, auth, from, recipients, msg)
}

func buildSMTPMessage(from string, recipients []string, subject, body, idempotencyKey string) []byte {
	messageID := ""
	if idempotencyKey != "" {
		messageID = "Message-ID: <" + url.QueryEscape(idempotencyKey) + "@sparkops>\r\n"
	}
	return []byte(
		"From: " + from + "\r\n" +
			"To: " + strings.Join(recipients, ",") + "\r\n" +
			"Subject: " + subject + "\r\n" +
			messageID + "\r\n" + body,
	)
}

func sendFeishuUsers(ctx context.Context, cfg ChannelConfig, targets, text, idempotencyKey string) error {
	return sendFeishuUsersWithClient(
		ctx, cfg, targets, text, idempotencyKey,
		&http.Client{Timeout: 8 * time.Second}, "https://open.feishu.cn",
	)
}

func sendFeishuUsersWithClient(
	ctx context.Context,
	cfg ChannelConfig,
	targets, text, idempotencyKey string,
	client *http.Client,
	baseURL string,
) error {
	if strings.TrimSpace(cfg.FeishuAppID) == "" || strings.TrimSpace(cfg.FeishuSecret) == "" {
		return nil
	}
	recipients := splitTargets(targets)
	if len(recipients) == 0 {
		return nil
	}
	token, err := feishuTenantToken(ctx, cfg.FeishuAppID, cfg.FeishuSecret, client, baseURL)
	if err != nil {
		return err
	}
	for _, target := range recipients {
		idType := "open_id"
		if strings.Contains(target, "@") {
			idType = "email"
		}
		content, _ := json.Marshal(map[string]string{"text": text})
		raw, _ := json.Marshal(map[string]string{
			"receive_id": target, "msg_type": "text", "content": string(content),
		})
		endpoint := strings.TrimRight(baseURL, "/") + "/open-apis/im/v1/messages?receive_id_type=" + idType
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(raw))
		if err != nil {
			return httperror.SanitizeNetwork(err)
		}
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("Content-Type", "application/json")
		if idempotencyKey != "" {
			req.Header.Set("X-SparkOps-Idempotency-Key", idempotencyKey)
		}
		resp, err := client.Do(req)
		if err != nil {
			return httperror.SanitizeNetwork(err)
		}
		var result struct {
			Code int    `json:"code"`
			Msg  string `json:"msg"`
		}
		decodeErr := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&result)
		closeErr := resp.Body.Close()
		if err := classifyNotificationHTTP(resp.StatusCode); err != nil {
			return err
		}
		if decodeErr != nil {
			return fmt.Errorf("decode Feishu response: %w", decodeErr)
		}
		if closeErr != nil {
			return fmt.Errorf("close Feishu response: %w", closeErr)
		}
		if result.Code != 0 {
			err := fmt.Errorf("Feishu message code %d: %s", result.Code, strings.TrimSpace(result.Msg))
			if result.Code == 99991663 || strings.Contains(strings.ToLower(result.Msg), "rate") {
				return err
			}
			return messaging.Permanent(err)
		}
	}
	return nil
}

func feishuTenantToken(ctx context.Context, appID, secret string, client *http.Client, baseURL string) (string, error) {
	raw, _ := json.Marshal(map[string]string{"app_id": appID, "app_secret": secret})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(baseURL, "/")+"/open-apis/auth/v3/tenant_access_token/internal", bytes.NewReader(raw))
	if err != nil {
		return "", httperror.SanitizeNetwork(err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		return "", httperror.SanitizeNetwork(err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return "", err
	}
	if err := classifyNotificationHTTP(resp.StatusCode); err != nil {
		return "", err
	}
	var payload struct {
		TenantAccessToken string `json:"tenant_access_token"`
		Code              int    `json:"code"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return "", err
	}
	if payload.Code != 0 {
		err := fmt.Errorf("Feishu token code %d", payload.Code)
		if payload.Code == 99991663 {
			return "", err
		}
		return "", messaging.Permanent(err)
	}
	if payload.TenantAccessToken == "" {
		return "", messaging.Permanent(fmt.Errorf("Feishu token is missing"))
	}
	return payload.TenantAccessToken, nil
}
