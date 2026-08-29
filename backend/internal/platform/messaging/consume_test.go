package messaging

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestValidateTopicAllowsDeployAndDLQ(t *testing.T) {
	require.NoError(t, ValidateTopic(TopicDeploy))
	require.NoError(t, ValidateTopic(DeadLetterTopic(TopicDeploy)))
	require.Equal(t, "deploy.sparkops.dlq", DeadLetterTopic(TopicDeploy))
}

func TestDecideConsumeAcksNilHandler(t *testing.T) {
	decision := DecideConsume(TopicDeploy, 0, nil)
	require.True(t, decision.Ack)
	require.Empty(t, decision.RepublishTopic)
	require.Empty(t, decision.DeadLetterTopic)
}

func TestDecideConsumeRepublishesUntilMaxRetry(t *testing.T) {
	handlerErr := errors.New("broker timeout")
	decision := DecideConsume(TopicDeploy, 0, handlerErr)
	require.Equal(t, ConsumeDecision{Ack: true, RepublishTopic: TopicDeploy, RepublishRetry: 1}, decision)

	decision = DecideConsume(TopicDeploy, 2, handlerErr)
	require.Equal(t, 3, decision.RepublishRetry)
	require.Empty(t, decision.DeadLetterTopic)
}

func TestDecideConsumePublishesDLQAfterMaxRetry(t *testing.T) {
	decision := DecideConsume(TopicDeploy, 3, errors.New("still failing"))
	require.True(t, decision.Ack)
	require.Empty(t, decision.RepublishTopic)
	require.Equal(t, DeadLetterTopic(TopicDeploy), decision.DeadLetterTopic)
}
