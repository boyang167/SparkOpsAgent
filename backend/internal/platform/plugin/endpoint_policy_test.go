package plugin

import (
	"context"
	"net"
	"testing"

	"github.com/stretchr/testify/require"
)

type staticResolver map[string][]net.IPAddr

func (r staticResolver) LookupIPAddr(_ context.Context, host string) ([]net.IPAddr, error) {
	return r[host], nil
}

func TestEndpointPolicyRejectsUnsafeAddressesAndResolvedPrivateIPs(t *testing.T) {
	policy, err := NewEndpointPolicy(nil, staticResolver{
		"internal.example": {{IP: net.ParseIP("10.0.0.2")}},
	})
	require.NoError(t, err)
	for _, endpoint := range []string{
		"http://127.0.0.1:9090",
		"http://169.254.169.254/latest",
		"http://10.1.2.3",
		"http://internal.example",
		"http://0.0.0.0",
		"http://224.0.0.1",
	} {
		require.ErrorIs(t, policy.Validate(context.Background(), endpoint), ErrEndpointNotAllowed, endpoint)
	}
}

func TestEndpointPolicyAllowsExplicitHostAndCIDR(t *testing.T) {
	hostPolicy, err := NewEndpointPolicy([]string{"127.0.0.1"}, nil)
	require.NoError(t, err)
	require.NoError(t, hostPolicy.Validate(context.Background(), "http://127.0.0.1:1234"))

	cidrPolicy, err := NewEndpointPolicy([]string{"10.20.0.0/16"}, nil)
	require.NoError(t, err)
	require.NoError(t, cidrPolicy.Validate(context.Background(), "https://10.20.1.2"))
}
