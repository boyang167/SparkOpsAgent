package model

import (
	"strings"

	"github.com/google/uuid"
)

const (
	NotifyEventCreated   = "created"
	NotifyEventPassed    = "passed"
	NotifyEventFailed    = "failed"
	NotifyEventTimeout   = "timeout"
	NotifyEventCancelled = "cancelled"
	NotifyEventChanged   = "changed"
	NotifyEventRejected  = "rejected"
	NotifyEventManual    = "manual"
	NotifyEventWaiting   = "waiting"

	NotifyChannelFeishuBot  = "feishu_bot"
	NotifyChannelFeishuApp  = "feishu_app"
	NotifyChannelFeishuUser = "feishu_user"
	NotifyChannelDingTalk   = "dingtalk"
	NotifyChannelWeCom      = "wecom"
	NotifyChannelTeams      = "teams"
	NotifyChannelMail       = "mail"
	NotifyChannelWebhook    = "webhook"
)

type WorkflowNotification struct {
	ID         string   `json:"id"`
	Enabled    *bool    `json:"enabled,omitempty"`
	Events     []string `json:"events"`
	Channel    string   `json:"channel"`
	WebhookURL string   `json:"webhook_url,omitempty"`
	Targets    string   `json:"targets,omitempty"`
}

func (n WorkflowNotification) Active() bool {
	return n.Enabled == nil || *n.Enabled
}

func NormalizeNotifications(items []WorkflowNotification) []WorkflowNotification {
	if len(items) == 0 {
		return []WorkflowNotification{}
	}
	out := make([]WorkflowNotification, 0, len(items))
	seen := map[string]struct{}{}
	for _, item := range items {
		item.ID = strings.TrimSpace(item.ID)
		if item.ID == "" {
			item.ID = uuid.NewString()
		}
		if _, ok := seen[item.ID]; ok {
			continue
		}
		channel := strings.TrimSpace(item.Channel)
		if !knownNotifyChannel(channel) {
			continue
		}
		events := uniqueNotifyEvents(item.Events)
		if len(events) == 0 {
			continue
		}
		seen[item.ID] = struct{}{}
		item.Channel = channel
		item.Events = events
		item.WebhookURL = strings.TrimSpace(item.WebhookURL)
		item.Targets = strings.TrimSpace(item.Targets)
		out = append(out, item)
	}
	return out
}

func knownNotifyChannel(channel string) bool {
	switch channel {
	case NotifyChannelFeishuBot, NotifyChannelFeishuApp, NotifyChannelFeishuUser,
		NotifyChannelDingTalk, NotifyChannelWeCom, NotifyChannelTeams,
		NotifyChannelMail, NotifyChannelWebhook:
		return true
	default:
		return false
	}
}

func uniqueNotifyEvents(events []string) []string {
	out := make([]string, 0, len(events))
	seen := map[string]struct{}{}
	for _, event := range events {
		event = strings.TrimSpace(event)
		if !knownNotifyEvent(event) {
			continue
		}
		if _, ok := seen[event]; ok {
			continue
		}
		seen[event] = struct{}{}
		out = append(out, event)
	}
	return out
}

func knownNotifyEvent(event string) bool {
	switch event {
	case NotifyEventCreated, NotifyEventPassed, NotifyEventFailed, NotifyEventTimeout,
		NotifyEventCancelled, NotifyEventChanged, NotifyEventRejected, NotifyEventManual, NotifyEventWaiting:
		return true
	default:
		return false
	}
}

func NotifyEventLabel(event string) string {
	switch event {
	case NotifyEventCreated:
		return "任务开始"
	case NotifyEventPassed:
		return "任务成功"
	case NotifyEventFailed:
		return "任务失败"
	case NotifyEventTimeout:
		return "任务超时"
	case NotifyEventCancelled:
		return "任务取消"
	case NotifyEventChanged:
		return "状态变更"
	case NotifyEventRejected:
		return "任务拒绝"
	case NotifyEventManual:
		return "等待手动执行"
	case NotifyEventWaiting:
		return "等待人工确认"
	default:
		return event
	}
}
