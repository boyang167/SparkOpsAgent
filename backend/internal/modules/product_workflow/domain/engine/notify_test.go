package engine

import (
	"encoding/json"
	"testing"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/product_workflow/domain/model"
	"github.com/stretchr/testify/require"
)

func TestNotificationBodyFeishu(t *testing.T) {
	raw, err := notificationBody(model.NotifyChannelFeishuBot, "hello", "failed", "构建", "run-1", "failed")
	require.NoError(t, err)
	var body map[string]any
	require.NoError(t, json.Unmarshal(raw, &body))
	require.Equal(t, "text", body["msg_type"])
}

func TestAllowedWebhookRejectsNonHTTP(t *testing.T) {
	require.False(t, allowedWebhook("file:///tmp/x"))
	require.True(t, allowedWebhook("https://open.feishu.cn/open-apis/bot/v2/hook/demo"))
}
