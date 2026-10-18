package httperror

import (
	"context"
	"errors"
	"net/url"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSanitizeNetworkRemovesURLAndPreservesClassification(t *testing.T) {
	const secret = "secret-query-token"
	const endpoint = "https://hooks.example.invalid/send?token=" + secret
	input := &url.Error{Op: "Post", URL: endpoint, Err: context.DeadlineExceeded}

	err := SanitizeNetwork(input)

	require.Error(t, err)
	require.ErrorIs(t, err, context.DeadlineExceeded)
	require.True(t, errors.As(err, new(interface{ Timeout() bool })))
	require.NotContains(t, err.Error(), secret)
	require.NotContains(t, err.Error(), endpoint)
}
