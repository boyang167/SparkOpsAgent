package model

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNormalizeNotificationsDropsUnknownChannel(t *testing.T) {
	got := NormalizeNotifications([]WorkflowNotification{
		{ID: "1", Events: []string{"failed", "nope"}, Channel: "webhook", WebhookURL: " https://example.com "},
		{ID: "2", Events: []string{"created"}, Channel: "sms"},
	})
	require.Len(t, got, 1)
	require.Equal(t, []string{NotifyEventFailed}, got[0].Events)
	require.Equal(t, "https://example.com", got[0].WebhookURL)
}
