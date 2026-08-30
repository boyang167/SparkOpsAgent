package plugin

import (
	"context"
	"errors"
	"net"
	"net/url"
	"strings"
)

var ErrEndpointNotAllowed = errors.New("endpoint is not allowed")

type EndpointPolicy interface {
	Validate(ctx context.Context, endpoint string) error
}

type IPResolver interface {
	LookupIPAddr(ctx context.Context, host string) ([]net.IPAddr, error)
}

type endpointPolicy struct {
	hosts    map[string]struct{}
	networks []*net.IPNet
	resolver IPResolver
}

func NewEndpointPolicy(allowed []string, resolver IPResolver) (EndpointPolicy, error) {
	p := &endpointPolicy{hosts: map[string]struct{}{}, resolver: resolver}
	if p.resolver == nil {
		p.resolver = net.DefaultResolver
	}
	for _, raw := range allowed {
		value := strings.TrimSpace(strings.ToLower(raw))
		if value == "" {
			continue
		}
		if _, network, err := net.ParseCIDR(value); err == nil {
			p.networks = append(p.networks, network)
			continue
		}
		if strings.ContainsAny(value, "/:@") {
			return nil, ErrEndpointNotAllowed
		}
		p.hosts[value] = struct{}{}
	}
	return p, nil
}

func NewEndpointPolicyFromCSV(raw string) (EndpointPolicy, error) {
	return NewEndpointPolicy(strings.Split(raw, ","), nil)
}

func (p *endpointPolicy) Validate(ctx context.Context, endpoint string) error {
	parsed, err := url.ParseRequestURI(endpoint)
	if err != nil || parsed.Host == "" || parsed.User != nil {
		return ErrEndpointNotAllowed
	}
	scheme := strings.ToLower(parsed.Scheme)
	if scheme != "http" && scheme != "https" {
		return ErrEndpointNotAllowed
	}
	host := strings.ToLower(parsed.Hostname())
	if host == "" {
		return ErrEndpointNotAllowed
	}
	if _, ok := p.hosts[host]; ok {
		return nil
	}
	if ip := net.ParseIP(host); ip != nil {
		if p.ipAllowed(ip) {
			return nil
		}
		return ErrEndpointNotAllowed
	}
	addresses, err := p.resolver.LookupIPAddr(ctx, host)
	if err != nil || len(addresses) == 0 {
		return ErrEndpointNotAllowed
	}
	for _, address := range addresses {
		if !p.ipAllowed(address.IP) {
			return ErrEndpointNotAllowed
		}
	}
	return nil
}

func (p *endpointPolicy) ipAllowed(ip net.IP) bool {
	for _, network := range p.networks {
		if network.Contains(ip) {
			return true
		}
	}
	return !(ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() ||
		ip.IsLinkLocalMulticast() || ip.IsUnspecified() || ip.IsMulticast())
}
