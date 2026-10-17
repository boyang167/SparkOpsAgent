package messaging

import (
	"strings"
	"testing"

	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/stretchr/testify/require"
)

func TestEventTopologyDeclaresQuorumMainRetryAndDLQ(t *testing.T) {
	declarations := topologyFor(Subscription{
		EventName:  "delivery.BuildCompleted",
		Version:    1,
		Subscriber: "notification.build-completed",
	})

	require.Equal(t, "sparkops.event.notification.build-completed.delivery-buildcompleted.v1", declarations.Main.Name)
	require.Equal(t, amqp.Table{"x-queue-type": "quorum"}, declarations.Main.Args)
	require.Equal(t, "delivery.BuildCompleted.v1", declarations.Main.RoutingKey)
	require.Len(t, declarations.Retry, 4)
	require.Equal(t, int32(5000), declarations.Retry[0].Args["x-message-ttl"])
	require.Equal(t, "", declarations.Retry[0].Args["x-dead-letter-exchange"])
	require.Equal(t, declarations.Main.Name, declarations.Retry[0].Args["x-dead-letter-routing-key"])
	require.Equal(t, "quorum", declarations.DeadLetter.Args["x-queue-type"])
}

func TestEventTopologyUsesFourStableRetryTiers(t *testing.T) {
	declarations := topologyFor(Subscription{
		EventName:  "delivery.BuildCompleted",
		Version:    2,
		Subscriber: "Audit Worker",
	})

	require.Equal(t, []int32{5_000, 30_000, 300_000, 1_800_000}, []int32{
		declarations.Retry[0].Args["x-message-ttl"].(int32),
		declarations.Retry[1].Args["x-message-ttl"].(int32),
		declarations.Retry[2].Args["x-message-ttl"].(int32),
		declarations.Retry[3].Args["x-message-ttl"].(int32),
	})
	for _, queue := range append(declarations.Retry, declarations.DeadLetter) {
		require.Regexp(t, `^[a-z0-9.-]+$`, queue.Name)
		require.Equal(t, "quorum", queue.Args["x-queue-type"])
	}
}

func TestValidateEventTopologiesRejectsNamesOverLimit(t *testing.T) {
	err := validateEventTopologies([]Subscription{{
		EventName:  "delivery.BuildCompleted",
		Version:    1,
		Subscriber: strings.Repeat("subscriber", 30),
	}})

	require.ErrorContains(t, err, "240")
}

func TestValidateEventTopologiesRejectsNormalizedNameCollision(t *testing.T) {
	err := validateEventTopologies([]Subscription{
		{EventName: "delivery.BuildCompleted", Version: 1, Subscriber: "audit_worker"},
		{EventName: "delivery.BuildCompleted", Version: 1, Subscriber: "audit-worker"},
	})

	require.ErrorContains(t, err, "collision")
}
