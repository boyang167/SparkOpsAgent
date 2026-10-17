package engine

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"

	workflowoutbound "github.com/sparkops/sparkops-agent/backend/internal/modules/product_workflow/infrastructure/outbound"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/messaging"
)

func TestMailOperationRejectsMissingConfigurationPermanently(t *testing.T) {
	previous := currentChannels()
	t.Cleanup(func() { ConfigureChannels(previous) })
	ConfigureChannels(ChannelConfig{})

	err := SendMailOperation(context.Background(), workflowoutbound.NotificationPayload{
		Targets: "ops@example.com", WorkflowName: "release",
	})

	require.Error(t, err)
	require.True(t, messaging.IsPermanent(err))
}

func TestMailOperationRejectsMissingRecipientsPermanently(t *testing.T) {
	previous := currentChannels()
	t.Cleanup(func() { ConfigureChannels(previous) })
	ConfigureChannels(ChannelConfig{SMTPHost: "smtp.example.invalid", SMTPFrom: "sparkops@example.com"})

	err := SendMailOperation(context.Background(), workflowoutbound.NotificationPayload{WorkflowName: "release"})

	require.Error(t, err)
	require.True(t, messaging.IsPermanent(err))
}

func TestFeishuOperationParsesHTTP200BusinessError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/open-apis/auth/v3/tenant_access_token/internal" {
			_, _ = w.Write([]byte(`{"code":0,"tenant_access_token":"token"}`))
			return
		}
		_, _ = w.Write([]byte(`{"code":99991663,"msg":"rate limited"}`))
	}))
	defer server.Close()

	err := sendFeishuUsersWithClient(
		context.Background(),
		ChannelConfig{FeishuAppID: "app", FeishuSecret: "secret"},
		"ou_1", "hello", "event:owner:type:target",
		server.Client(), server.URL,
	)

	require.Error(t, err)
	require.False(t, messaging.IsPermanent(err))
}

func TestFeishuTokenRejectsHTTP200NonzeroCodeEvenWhenTokenPresent(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"code":10003,"tenant_access_token":"must-not-be-used"}`))
	}))
	defer server.Close()

	_, err := feishuTenantToken(
		context.Background(), "app", "secret", server.Client(), server.URL,
	)

	require.Error(t, err)
	require.True(t, messaging.IsPermanent(err))
}
