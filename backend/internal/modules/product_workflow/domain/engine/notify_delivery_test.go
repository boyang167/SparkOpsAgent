package engine

import (
	"bytes"
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

func TestSMTPMessageIDIsStableForOperationIdempotencyKey(t *testing.T) {
	key := "event-1:subscriber:product-workflow.mail.send.v1:mail-a"

	first := buildSMTPMessage("from@example.com", []string{"ops@example.com"}, "subject", "body", key)
	second := buildSMTPMessage("from@example.com", []string{"ops@example.com"}, "subject", "body", key)

	require.Equal(t, first, second)
	require.Equal(t, 1, bytes.Count(first, []byte("Message-ID:")))
	require.Contains(t, string(first), "Message-ID: <event-1%3Asubscriber%3Aproduct-workflow.mail.send.v1%3Amail-a@sparkops>")
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

func TestFeishuTokenClassifiesHTTPErrorBeforeParsingBody(t *testing.T) {
	for _, test := range []struct {
		name      string
		status    int
		body      string
		permanent bool
	}{
		{name: "HTML unauthorized", status: http.StatusUnauthorized, body: "<html>unauthorized</html>", permanent: true},
		{name: "empty unauthorized", status: http.StatusUnauthorized, permanent: true},
		{name: "rate limited", status: http.StatusTooManyRequests, body: "<html>busy</html>"},
		{name: "server error", status: http.StatusInternalServerError},
	} {
		t.Run(test.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(test.status)
				_, _ = w.Write([]byte(test.body))
			}))
			defer server.Close()

			_, err := feishuTenantToken(
				context.Background(), "app", "secret", server.Client(), server.URL,
			)

			require.Error(t, err)
			require.Equal(t, test.permanent, messaging.IsPermanent(err))
		})
	}
}
