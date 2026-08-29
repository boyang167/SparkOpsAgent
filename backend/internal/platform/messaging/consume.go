package messaging

const (
	TopicDeploy     = "deploy.sparkops"
	HeaderRetry     = "x-sparkops-retry"
	MaxConsumeRetry = 3
)

func DeadLetterTopic(topic string) string {
	return topic + ".dlq"
}

type ConsumeDecision struct {
	Ack             bool
	RepublishTopic  string
	RepublishRetry  int
	DeadLetterTopic string
}

func DecideConsume(topic string, retry int, handlerErr error) ConsumeDecision {
	if handlerErr == nil {
		return ConsumeDecision{Ack: true}
	}
	next := retry + 1
	if next <= MaxConsumeRetry {
		return ConsumeDecision{Ack: true, RepublishTopic: topic, RepublishRetry: next}
	}
	return ConsumeDecision{Ack: true, DeadLetterTopic: DeadLetterTopic(topic)}
}
