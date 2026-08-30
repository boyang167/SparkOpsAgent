package plugin

import (
	"context"
	"errors"
	"net"
	"net/http"
	"sync"
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

type recordingDialer struct {
	mu      sync.Mutex
	address string
}

func (d *recordingDialer) DialContext(_ context.Context, _, address string) (net.Conn, error) {
	d.mu.Lock()
	d.address = address
	d.mu.Unlock()
	return nil, errors.New("dial stopped by test")
}

func TestEndpointPolicyPinsResolvedAddressDespiteLaterDNSChange(t *testing.T) {
	resolver := staticResolver{"metrics.example": {{IP: net.ParseIP("203.0.113.10")}}}
	dialer := &recordingDialer{}
	policy, err := NewEndpointPolicyWithDialer(nil, resolver, dialer.DialContext)
	require.NoError(t, err)

	client, err := policy.PinnedClient(context.Background(), "https://metrics.example/query", &http.Client{})
	require.NoError(t, err)
	resolver["metrics.example"] = []net.IPAddr{{IP: net.ParseIP("10.0.0.9")}}
	transport := client.Transport.(*http.Transport)
	_, _ = transport.DialContext(context.Background(), "tcp", "metrics.example:443")

	require.Equal(t, "203.0.113.10:443", dialer.address)
	require.Equal(t, "metrics.example", transport.TLSClientConfig.ServerName)
	require.ErrorIs(t, policy.Validate(context.Background(), "https://metrics.example"), ErrEndpointNotAllowed)
}

func TestEndpointPolicyPinsAllowlistedPrivateIPv6WithPort(t *testing.T) {
	resolver := staticResolver{"internal.example": {{IP: net.ParseIP("fd00::20")}}}
	dialer := &recordingDialer{}
	policy, err := NewEndpointPolicyWithDialer([]string{"internal.example"}, resolver, dialer.DialContext)
	require.NoError(t, err)

	client, err := policy.PinnedClient(context.Background(), "https://internal.example:9443", &http.Client{})
	require.NoError(t, err)
	transport := client.Transport.(*http.Transport)
	_, _ = transport.DialContext(context.Background(), "tcp", "internal.example:9443")

	require.Equal(t, "[fd00::20]:9443", dialer.address)
	require.Equal(t, "internal.example", transport.TLSClientConfig.ServerName)
}
