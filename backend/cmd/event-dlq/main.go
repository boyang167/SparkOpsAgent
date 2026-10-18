package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"os/signal"
	"regexp"
	"strings"

	"github.com/google/uuid"
	amqp "github.com/rabbitmq/amqp091-go"
	agentevent "github.com/sparkops/sparkops-agent/backend/internal/modules/agent/domain/event"
	agentruntimeevent "github.com/sparkops/sparkops-agent/backend/internal/modules/agent_runtime/domain/event"
	deliveryevent "github.com/sparkops/sparkops-agent/backend/internal/modules/delivery/domain/event"
	identityevent "github.com/sparkops/sparkops-agent/backend/internal/modules/identity/domain/event"
	productworkflowevent "github.com/sparkops/sparkops-agent/backend/internal/modules/product_workflow/domain/event"
	workflowevent "github.com/sparkops/sparkops-agent/backend/internal/modules/workflow/domain/event"
	workspaceevent "github.com/sparkops/sparkops-agent/backend/internal/modules/workspace_app/domain/event"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/messaging"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/events"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/integration"
)

const defaultEventExchange = "sparkops.events"

var dlqQueueName = regexp.MustCompile(`^sparkops\.event\.[a-z0-9.-]+\.v[1-9][0-9]*\.dlq$`)

type options struct {
	operation string
	queue     string
	limit     int
}

type queueClient interface {
	QueueInspect(string) (amqp.Queue, error)
	Get(string, bool) (amqp.Delivery, bool, error)
	Close() error
}

type eventPublisher interface {
	Publish(context.Context, events.Envelope) error
	Close() error
}

type realQueueClient struct {
	connection *amqp.Connection
	channel    *amqp.Channel
}

func (c *realQueueClient) QueueInspect(name string) (amqp.Queue, error) {
	return c.channel.QueueInspect(name)
}

func (c *realQueueClient) Get(name string, autoAck bool) (amqp.Delivery, bool, error) {
	return c.channel.Get(name, autoAck)
}

func (c *realQueueClient) Close() error {
	return errors.Join(c.channel.Close(), c.connection.Close())
}

func parseOptions(args []string) (options, error) {
	if len(args) == 0 {
		return options{}, fmt.Errorf("operation is required: inspect or replay")
	}
	operation := args[0]
	if operation != "inspect" && operation != "replay" {
		return options{}, fmt.Errorf("operation must be inspect or replay")
	}
	flags := flag.NewFlagSet("event-dlq "+operation, flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	queue := flags.String("queue", "", "explicit event DLQ queue name")
	limit := flags.Int("limit", 0, "maximum messages to replay")
	if err := flags.Parse(args[1:]); err != nil {
		return options{}, err
	}
	if flags.NArg() != 0 {
		return options{}, fmt.Errorf("unexpected positional arguments")
	}
	if strings.TrimSpace(*queue) == "" {
		return options{}, fmt.Errorf("--queue is required")
	}
	if !dlqQueueName.MatchString(*queue) || strings.Contains(*queue, "..") {
		return options{}, fmt.Errorf("queue must be an explicit SparkOps event DLQ name")
	}
	if operation == "replay" && *limit <= 0 {
		return options{}, fmt.Errorf("replay requires a positive --limit")
	}
	if operation == "inspect" && *limit != 0 {
		return options{}, fmt.Errorf("--limit is valid only for replay")
	}
	return options{operation: operation, queue: *queue, limit: *limit}, nil
}

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()
	if err := run(ctx, os.Args[1:], os.Stdout); err != nil {
		log.Fatal(err)
	}
}

func run(ctx context.Context, args []string, output io.Writer) error {
	opts, err := parseOptions(args)
	if err != nil {
		return err
	}
	amqpURL := strings.TrimSpace(os.Getenv("AMQP_URL"))
	if amqpURL == "" {
		return fmt.Errorf("AMQP_URL is required")
	}
	client, err := openQueueClient(amqpURL)
	if err != nil {
		return err
	}
	defer client.Close()

	if opts.operation == "inspect" {
		queue, inspectErr := client.QueueInspect(opts.queue)
		if inspectErr != nil {
			return fmt.Errorf("passively inspect queue %q: %w", opts.queue, inspectErr)
		}
		_, err = fmt.Fprintf(
			output,
			"queue=%s messages=%d consumers=%d\n",
			queue.Name,
			queue.Messages,
			queue.Consumers,
		)
		return err
	}

	registry, err := productionRegistry()
	if err != nil {
		return err
	}
	eventExchange := strings.TrimSpace(os.Getenv("AMQP_EVENT_EXCHANGE"))
	if eventExchange == "" {
		eventExchange = defaultEventExchange
	}
	publisher, err := messaging.NewAMQPEventTransport(amqpURL, nil, eventExchange)
	if err != nil {
		return err
	}
	defer publisher.Close()
	return replay(ctx, client, publisher, registry, opts.queue, opts.limit, output)
}

func openQueueClient(amqpURL string) (queueClient, error) {
	connection, err := amqp.Dial(amqpURL)
	if err != nil {
		return nil, fmt.Errorf("dial RabbitMQ: %w", err)
	}
	channel, err := connection.Channel()
	if err != nil {
		_ = connection.Close()
		return nil, fmt.Errorf("open RabbitMQ channel: %w", err)
	}
	return &realQueueClient{connection: connection, channel: channel}, nil
}

func replay(
	ctx context.Context,
	client queueClient,
	publisher eventPublisher,
	registry *events.Registry,
	queue string,
	limit int,
	output io.Writer,
) error {
	replayed := 0
	for replayed < limit {
		delivery, ok, err := client.Get(queue, false)
		if err != nil {
			return fmt.Errorf("read DLQ %q: %w", queue, err)
		}
		if !ok {
			break
		}
		var envelope events.Envelope
		if err := json.Unmarshal(delivery.Body, &envelope); err != nil {
			return fmt.Errorf("validate DLQ delivery %d: decode envelope: %w", delivery.DeliveryTag, err)
		}
		if err := validateReplayEnvelope(envelope); err != nil {
			return fmt.Errorf("validate DLQ delivery %d: %w", delivery.DeliveryTag, err)
		}
		if _, err := registry.Decode(envelope); err != nil {
			return fmt.Errorf("validate DLQ delivery %d: %w", delivery.DeliveryTag, err)
		}
		if err := publisher.Publish(ctx, envelope); err != nil {
			return fmt.Errorf("republish event %s: %w", envelope.ID, err)
		}
		if err := delivery.Ack(false); err != nil {
			return fmt.Errorf("ack replayed event %s: %w", envelope.ID, err)
		}
		replayed++
	}
	_, err := fmt.Fprintf(output, "queue=%s replayed=%d limit=%d\n", queue, replayed, limit)
	return err
}

func validateReplayEnvelope(envelope events.Envelope) error {
	if _, err := uuid.Parse(envelope.ID); err != nil {
		return fmt.Errorf("event id must be a UUID")
	}
	if strings.TrimSpace(envelope.Type) == "" || envelope.Version <= 0 {
		return fmt.Errorf("event type and positive version are required")
	}
	if envelope.OccurredAt.IsZero() || len(envelope.Payload) == 0 || !json.Valid(envelope.Payload) {
		return fmt.Errorf("event occurrence time and valid payload are required")
	}
	return nil
}

func productionRegistry() (*events.Registry, error) {
	registry := events.NewRegistry()
	registrations := []func(*events.Registry) error{
		identityevent.Register,
		integration.RegisterWorkspaceCreated,
		workspaceevent.Register,
		integration.RegisterSecretUpdated,
		agentevent.Register,
		deliveryevent.Register,
		workflowevent.Register,
		productworkflowevent.Register,
		agentruntimeevent.Register,
	}
	for _, register := range registrations {
		if err := register(registry); err != nil {
			return nil, fmt.Errorf("register production event codec: %w", err)
		}
	}
	return registry, nil
}
