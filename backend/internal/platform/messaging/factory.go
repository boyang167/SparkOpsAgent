package messaging

import (
	"fmt"

	"github.com/sparkops/sparkops-agent/backend/internal/platform/config"
)

type Factory struct {
	Bus   Bus
	Queue Queue
}

func NewFromConfig(cfg *config.Config) (*Factory, error) {
	bus := NewInMemoryBus()

	driver := cfg.Messaging.Driver
	if driver == "" {
		driver = "inmemory"
	}

	var queue Queue
	var err error
	switch driver {
	case "inmemory":
		queue = NewNoopQueue()
	case "amqp":
		queue, err = NewAMQPQueue(AMQPConfig{
			URL:          cfg.Messaging.AMQP.URL,
			Exchange:     cfg.Messaging.AMQP.Exchange,
			ExchangeType: cfg.Messaging.AMQP.ExchangeType,
		})
		if err != nil {
			return nil, fmt.Errorf("create amqp queue: %w", err)
		}
	default:
		return nil, fmt.Errorf("unknown messaging driver: %s", driver)
	}

	return &Factory{Bus: bus, Queue: queue}, nil
}
