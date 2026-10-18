package service

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/product/domain/model"
	productoutbound "github.com/sparkops/sparkops-agent/backend/internal/modules/product/infrastructure/outbound"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/messaging"
	messagingpersistence "github.com/sparkops/sparkops-agent/backend/internal/platform/messaging/persistence"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

func TestReleasePlanNotifiersSanitizeInvalidRequestURLs(t *testing.T) {
	const (
		secretPath  = "invalid-path-secret"
		secretQuery = "invalid-query-secret"
		endpoint    = "https://example.com/" + secretPath + "/%zz?token=" + secretQuery
	)

	hookPlan := releasePlanHookTestPlan(endpoint)
	err := NewReleasePlanHookNotifier("https://sparkops.example").Send(
		context.Background(), hookPlan, model.ReleasePlanHookEventAllDone,
	)
	require.Error(t, err)
	require.True(t, messaging.IsPermanent(err))
	assertReleasePlanNetworkErrorSanitized(t, err, endpoint, secretPath, secretQuery)

	approvalPlan := releasePlanApprovalTestPlan(endpoint)
	err = NewReleasePlanApprovalNotifier("https://sparkops.example").
		NotifyApproval(context.Background(), approvalPlan)
	require.Error(t, err)
	require.True(t, messaging.IsPermanent(err))
	assertReleasePlanNetworkErrorSanitized(t, err, endpoint, secretPath, secretQuery)
}

func TestReleasePlanOperationNetworkErrorsAreSanitizedInHandlerPersistenceAndLogs(t *testing.T) {
	tests := []struct {
		name          string
		operationType string
		payload       any
		register      func(t *testing.T, registry messaging.OperationRegistry, client *http.Client)
		wantTimeout   bool
	}{
		{
			name:          "webhook DNS temporary error",
			operationType: productoutbound.OperationTypeReleasePlanWebhookSend,
			payload: productoutbound.ReleasePlanWebhookPayload{
				PlanID: kernel.NewReleasePlanID().String(),
				Event:  string(model.ReleasePlanHookEventAllDone),
			},
			register: func(t *testing.T, registry messaging.OperationRegistry, client *http.Client) {
				plan := releasePlanHookTestPlan(releasePlanSecretEndpoint)
				notifier := NewReleasePlanHookNotifier("https://sparkops.example")
				notifier.client = client
				require.NoError(t, registry.Register(
					productoutbound.OperationTypeReleasePlanWebhookSend,
					productoutbound.NewReleasePlanWebhookOperation(
						func(ctx context.Context, _ kernel.ReleasePlanID, event, _, key string) error {
							return notifier.SendWithIdempotency(
								ctx, plan, model.ReleasePlanHookEvent(event), key,
							)
						},
					),
				))
			},
		},
		{
			name:          "approval timeout error",
			operationType: productoutbound.OperationTypeReleasePlanApprovalSend,
			payload: productoutbound.ReleasePlanApprovalPayload{
				PlanID: kernel.NewReleasePlanID().String(),
			},
			register: func(t *testing.T, registry messaging.OperationRegistry, client *http.Client) {
				plan := releasePlanApprovalTestPlan(releasePlanSecretEndpoint)
				notifier := NewReleasePlanApprovalNotifier("https://sparkops.example")
				notifier.client = client
				require.NoError(t, registry.Register(
					productoutbound.OperationTypeReleasePlanApprovalSend,
					productoutbound.NewReleasePlanApprovalOperation(
						func(ctx context.Context, _ kernel.ReleasePlanID, key string) error {
							return notifier.NotifyApprovalWithIdempotency(ctx, plan, key)
						},
					),
				))
			},
			wantTimeout: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var transportErr error = &net.DNSError{
				Err:         "temporary resolver failure",
				Name:        "webhook.invalid",
				IsTemporary: true,
			}
			if tt.wantTimeout {
				transportErr = context.DeadlineExceeded
			}
			client := &http.Client{Transport: releasePlanRoundTripFunc(
				func(*http.Request) (*http.Response, error) {
					return nil, transportErr
				},
			)}

			registry := messaging.NewOperationRegistry()
			tt.register(t, registry, client)
			payload, err := json.Marshal(tt.payload)
			require.NoError(t, err)

			const operationID = "release-plan-network-operation"
			now := time.Now().UTC()
			store := &releasePlanOperationStore{
				record: messagingpersistence.OperationRecord{
					ID:             operationID,
					EventID:        "event-network",
					Subscriber:     "product.release-plan",
					OperationType:  tt.operationType,
					IdempotencyKey: "event-network:product.release-plan:" + tt.operationType + ":plan-1",
					Payload:        payload,
					Attempts:       1,
					NextAttemptAt:  now,
					CreatedAt:      now,
				},
			}

			core, logs := observer.New(zap.WarnLevel)
			dispatcher := messaging.NewOperationDispatcher(
				store,
				registry,
				messaging.OperationDispatcherConfig{Owner: "worker-a", BatchSize: 1},
				nil,
				nil,
				zap.New(core),
			)

			handlerErr := dispatcher.DispatchOnce(context.Background())
			require.Error(t, handlerErr)
			assertReleasePlanNetworkErrorSanitized(
				t, handlerErr, releasePlanSecretEndpoint,
				releasePlanSecretPath, releasePlanSecretQuery,
			)
			var networkErr interface {
				Timeout() bool
				Temporary() bool
			}
			require.ErrorAs(t, handlerErr, &networkErr)
			require.Equal(t, tt.wantTimeout, networkErr.Timeout())
			require.True(t, networkErr.Temporary())

			assertReleasePlanNetworkStringSanitized(
				t, store.lastError, releasePlanSecretEndpoint,
				releasePlanSecretPath, releasePlanSecretQuery,
			)

			entries := logs.FilterMessage("operation execution failed").All()
			require.Len(t, entries, 1)
			loggedError := fmt.Sprint(entries[0].ContextMap()["error"])
			assertReleasePlanNetworkStringSanitized(
				t, loggedError, releasePlanSecretEndpoint,
				releasePlanSecretPath, releasePlanSecretQuery,
			)
		})
	}
}

const (
	releasePlanSecretPath     = "private-webhook-secret"
	releasePlanSecretQuery    = "query-token-secret"
	releasePlanSecretEndpoint = "https://webhook.invalid/" + releasePlanSecretPath + "?token=" + releasePlanSecretQuery
)

func releasePlanHookTestPlan(endpoint string) *model.ReleasePlan {
	return &model.ReleasePlan{
		ID:   kernel.NewReleasePlanID(),
		Name: "release plan",
		HookSettings: model.ReleasePlanHookSettings{
			Enabled: true,
			Events: map[model.ReleasePlanHookEvent]model.ReleasePlanHookEventConfig{
				model.ReleasePlanHookEventAllDone: {Enabled: true, URL: endpoint},
			},
		},
		HookState: &model.ReleasePlanHookState{
			Event: model.ReleasePlanHookEventAllDone, Token: "callback-token", Status: "pending",
		},
	}
}

func releasePlanApprovalTestPlan(endpoint string) *model.ReleasePlan {
	return &model.ReleasePlan{
		ID:   kernel.NewReleasePlanID(),
		Name: "release plan",
		ApprovalConfig: model.ApprovalConfig{
			Enabled: true, Provider: model.ApprovalProviderFeishu, WebhookURL: endpoint,
			Instance: &model.ApprovalInstance{Token: "callback-token"},
		},
	}
}

func assertReleasePlanNetworkErrorSanitized(
	t *testing.T,
	err error,
	endpoint string,
	secrets ...string,
) {
	t.Helper()
	assertReleasePlanNetworkStringSanitized(t, err.Error(), endpoint, secrets...)
}

func assertReleasePlanNetworkStringSanitized(
	t *testing.T,
	value string,
	endpoint string,
	secrets ...string,
) {
	t.Helper()
	require.NotContains(t, value, endpoint)
	for _, secret := range secrets {
		require.NotContains(t, value, secret)
	}
}

type releasePlanRoundTripFunc func(*http.Request) (*http.Response, error)

func (f releasePlanRoundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

type releasePlanOperationStore struct {
	record    messagingpersistence.OperationRecord
	claimed   bool
	lastError string
}

func (*releasePlanOperationStore) Insert(context.Context, messagingpersistence.OperationRecord) error {
	return nil
}

func (s *releasePlanOperationStore) Claim(
	_ context.Context,
	owner string,
	_ int,
	_ time.Time,
	_ time.Duration,
) ([]messagingpersistence.OperationRecord, error) {
	if s.claimed {
		return nil, nil
	}
	s.claimed = true
	s.record.LockedBy = owner
	return []messagingpersistence.OperationRecord{s.record}, nil
}

func (*releasePlanOperationStore) MarkCompleted(context.Context, string, string, time.Time) error {
	return nil
}

func (s *releasePlanOperationStore) MarkFailed(
	_ context.Context,
	_ string,
	_ string,
	reason string,
	_ time.Time,
) error {
	s.lastError = reason
	return nil
}

func (s *releasePlanOperationStore) Reschedule(
	_ context.Context,
	_ string,
	_ string,
	reason string,
	_ time.Time,
) error {
	s.lastError = reason
	return nil
}

func (*releasePlanOperationStore) Stats(
	context.Context,
	time.Time,
) (messagingpersistence.OperationStats, error) {
	return messagingpersistence.OperationStats{}, nil
}
