package command

import (
	"net/url"
	"strings"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/delivery/domain"
)

const (
	WebhookEventBuild    = "Build"
	WebhookEventDeploy   = "Deploy"
	WebhookEventRollback = "Rollback"
)

var AllowedWebhookEvents = []string{WebhookEventBuild, WebhookEventDeploy, WebhookEventRollback}

func NormalizeWebhookEvents(raw []string) ([]string, error) {
	seen := map[string]struct{}{}
	out := make([]string, 0, len(raw))
	for _, item := range raw {
		name := strings.TrimSpace(item)
		if name == "" {
			continue
		}
		if !isAllowedWebhookEvent(name) {
			return nil, domain.ErrInvalidWebhook
		}
		if _, ok := seen[name]; ok {
			continue
		}
		seen[name] = struct{}{}
		out = append(out, name)
	}
	if len(out) == 0 {
		return nil, domain.ErrInvalidWebhook
	}
	return out, nil
}

func isAllowedWebhookEvent(name string) bool {
	for _, item := range AllowedWebhookEvents {
		if item == name {
			return true
		}
	}
	return false
}

func ValidateWebhookURL(raw string) (string, error) {
	value := strings.TrimSpace(raw)
	parsed, err := url.Parse(value)
	if err != nil || parsed.Host == "" {
		return "", domain.ErrInvalidWebhook
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return "", domain.ErrInvalidWebhook
	}
	return value, nil
}

func HookSubscribes(events []string, name string) bool {
	for _, item := range events {
		if item == name {
			return true
		}
	}
	return false
}
