package messaging

import (
	"fmt"
	"regexp"
	"strings"

	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/events"
)

const (
	eventExchangeName = "sparkops.events"
	maxEventQueueName = 240
)

var invalidQueueNamePart = regexp.MustCompile(`[^a-z0-9.-]+`)

type eventQueueDeclaration struct {
	Name       string
	RoutingKey string
	Args       amqp.Table
}

type eventTopology struct {
	Main       eventQueueDeclaration
	Retry      []eventQueueDeclaration
	DeadLetter eventQueueDeclaration
}

func topologyFor(subscription Subscription) eventTopology {
	subscriber := normalizeQueueNamePart(subscription.Subscriber, true)
	eventName := normalizeQueueNamePart(subscription.EventName, false)
	mainName := fmt.Sprintf(
		"sparkops.event.%s.%s.v%d",
		subscriber,
		eventName,
		subscription.Version,
	)
	routingKey := fmt.Sprintf("%s.v%d", subscription.EventName, subscription.Version)
	quorum := func() amqp.Table {
		return amqp.Table{"x-queue-type": "quorum"}
	}

	retryTiers := []struct {
		suffix string
		ttl    int32
	}{
		{suffix: "5s", ttl: 5_000},
		{suffix: "30s", ttl: 30_000},
		{suffix: "5m", ttl: 300_000},
		{suffix: "30m", ttl: 1_800_000},
	}
	retry := make([]eventQueueDeclaration, 0, len(retryTiers))
	for _, tier := range retryTiers {
		retry = append(retry, eventQueueDeclaration{
			Name: mainName + ".retry." + tier.suffix,
			Args: amqp.Table{
				"x-queue-type":              "quorum",
				"x-message-ttl":             tier.ttl,
				"x-dead-letter-exchange":    "",
				"x-dead-letter-routing-key": mainName,
			},
		})
	}

	return eventTopology{
		Main: eventQueueDeclaration{
			Name:       mainName,
			RoutingKey: routingKey,
			Args:       quorum(),
		},
		Retry: retry,
		DeadLetter: eventQueueDeclaration{
			Name: mainName + ".dlq",
			Args: quorum(),
		},
	}
}

func normalizeQueueNamePart(value string, preserveDots bool) string {
	value = strings.ToLower(value)
	if !preserveDots {
		value = strings.ReplaceAll(value, ".", "-")
	}
	value = invalidQueueNamePart.ReplaceAllString(value, "-")
	return strings.Trim(value, ".-")
}

// ValidateEventDeadLetterQueue validates an exact production DLQ identity and
// returns its corresponding main queue.
func ValidateEventDeadLetterQueue(queue string, envelope events.Envelope) (string, error) {
	if !stableEventTypeName.MatchString(envelope.Type) {
		return "", fmt.Errorf("event type has invalid stable event name syntax")
	}
	if envelope.Version <= 0 {
		return "", fmt.Errorf("event version must be positive")
	}
	const placeholder = "sparkops-dlq-subscriber"
	template := topologyFor(Subscription{
		EventName: envelope.Type, Version: envelope.Version, Subscriber: placeholder,
	})
	prefix := "sparkops.event."
	suffix := strings.TrimPrefix(template.DeadLetter.Name, prefix+placeholder)
	if !strings.HasPrefix(queue, prefix) || !strings.HasSuffix(queue, suffix) {
		return "", fmt.Errorf(
			"DLQ queue %q does not match envelope %s.v%d",
			queue,
			envelope.Type,
			envelope.Version,
		)
	}
	subscriber := strings.TrimSuffix(strings.TrimPrefix(queue, prefix), suffix)
	if subscriber == "" ||
		strings.Contains(subscriber, "..") ||
		normalizeQueueNamePart(subscriber, true) != subscriber {
		return "", fmt.Errorf("DLQ queue contains an invalid subscriber identity")
	}
	subscription := Subscription{
		EventName: envelope.Type, Version: envelope.Version, Subscriber: subscriber,
	}
	if err := validateEventTopologies([]Subscription{subscription}); err != nil {
		return "", err
	}
	topology := topologyFor(subscription)
	if queue != topology.DeadLetter.Name {
		return "", fmt.Errorf("DLQ queue %q is not an exact production dead-letter queue", queue)
	}
	return topology.Main.Name, nil
}

func validateEventTopologies(subscriptions []Subscription) error {
	type subscriptionIdentity struct {
		eventName  string
		version    int
		subscriber string
	}
	owners := make(map[string]subscriptionIdentity, len(subscriptions))
	for _, subscription := range subscriptions {
		topology := topologyFor(subscription)
		queues := append(append([]eventQueueDeclaration{topology.Main}, topology.Retry...), topology.DeadLetter)
		for _, queue := range queues {
			if len(queue.Name) > maxEventQueueName {
				return fmt.Errorf("event queue name exceeds %d characters: %s", maxEventQueueName, queue.Name)
			}
		}
		identity := subscriptionIdentity{
			eventName:  subscription.EventName,
			version:    subscription.Version,
			subscriber: subscription.Subscriber,
		}
		if previous, exists := owners[topology.Main.Name]; exists && previous != identity {
			return fmt.Errorf(
				"event queue name collision between %q and %q",
				previous.subscriber,
				subscription.Subscriber,
			)
		}
		owners[topology.Main.Name] = identity
	}
	return nil
}
