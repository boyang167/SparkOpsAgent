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

type ReleasePlanApprovalNotifier struct {
	publicBase string
	client     *http.Client
}

func NewReleasePlanApprovalNotifier(publicBase string) *ReleasePlanApprovalNotifier {
	base := strings.TrimRight(strings.TrimSpace(publicBase), "/")
	return &ReleasePlanApprovalNotifier{
		publicBase: base,
		client:     &http.Client{Timeout: 8 * time.Second},
	}
}

func (n *ReleasePlanApprovalNotifier) NotifyApproval(ctx context.Context, plan *model.ReleasePlan) error {
	if n == nil || plan == nil || !plan.ApprovalConfig.UsesExternalApproval() {
		return nil
	}
	webhook := strings.TrimSpace(plan.ApprovalConfig.WebhookURL)
	if webhook == "" || plan.ApprovalConfig.Instance == nil || plan.ApprovalConfig.Instance.Token == "" {
		return nil
	}
	if !allowedWebhookURL(webhook) {
		return nil
	}
	approveURL := n.publicBase + "/api/v1/public/release-plans/approval/" + url.PathEscape(plan.ApprovalConfig.Instance.Token) + "/approve"
	rejectURL := n.publicBase + "/api/v1/public/release-plans/approval/" + url.PathEscape(plan.ApprovalConfig.Instance.Token) + "/reject"
	body, err := approvalBody(plan.ApprovalConfig.ProviderOrDefault(), plan.Name, plan.Description, approveURL, rejectURL)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, webhook, bytes.NewReader(body))
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
		return fmt.Errorf("approval webhook status %d", resp.StatusCode)
	}
	return nil
}

func approvalBody(provider model.ApprovalProvider, name, description, approveURL, rejectURL string) ([]byte, error) {
	title := "发布计划审批"
	summary := fmt.Sprintf("**%s** 待审批", strings.TrimSpace(name))
	if strings.TrimSpace(description) != "" {
		summary += "\n" + strings.TrimSpace(description)
	}
	switch provider {
	case model.ApprovalProviderFeishu:
		return json.Marshal(map[string]any{
			"msg_type": "interactive",
			"card": map[string]any{
				"header": map[string]any{
					"title": map[string]any{"tag": "plain_text", "content": title},
				},
				"elements": []any{
					map[string]any{"tag": "div", "text": map[string]any{"tag": "lark_md", "content": summary}},
					map[string]any{
						"tag": "action",
						"actions": []any{
							map[string]any{
								"tag": "button", "type": "primary",
								"text": map[string]any{"tag": "plain_text", "content": "批准"},
								"url": approveURL,
							},
							map[string]any{
								"tag": "button",
								"text": map[string]any{"tag": "plain_text", "content": "拒绝"},
								"url": rejectURL,
							},
						},
					},
				},
			},
		})
	case model.ApprovalProviderDingTalk:
		return json.Marshal(map[string]any{
			"msgtype": "actionCard",
			"actionCard": map[string]any{
				"title":          title,
				"text":           summary,
				"btnOrientation": "0",
				"btns": []any{
					map[string]any{"title": "批准", "actionURL": approveURL},
					map[string]any{"title": "拒绝", "actionURL": rejectURL},
				},
			},
		})
	default:
		text := summary + "\n批准: " + approveURL + "\n拒绝: " + rejectURL
		return json.Marshal(map[string]any{
			"msg_type": "text",
			"content":  map[string]string{"text": text},
		})
	}
}

func allowedWebhookURL(raw string) bool {
	parsed, err := url.Parse(raw)
	if err != nil {
		return false
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return false
	}
	return parsed.Host != ""
}
