package plugin

import (
	"context"
	"crypto/tls"
	"errors"
	"net"
	"net/http"
	"net/url"
	"strings"
)

var ErrEndpointNotAllowed = errors.New("endpoint is not allowed")

type EndpointPolicy interface {
	Validate(ctx context.Context, endpoint string) error
	PinnedClient(ctx context.Context, endpoint string, template *http.Client) (*http.Client, error)
}

type IPResolver interface {
	LookupIPAddr(ctx context.Context, host string) ([]net.IPAddr, error)
}

type endpointPolicy struct {
	hosts    map[string]struct{}
	networks []*net.IPNet
	resolver IPResolver
	dialer   func(context.Context, string, string) (net.Conn, error)
}

func NewEndpointPolicy(allowed []string, resolver IPResolver) (EndpointPolicy, error) {
	return NewEndpointPolicyWithDialer(allowed, resolver, nil)
}

func NewEndpointPolicyWithDialer(
	allowed []string,
	resolver IPResolver,
	dialer func(context.Context, string, string) (net.Conn, error),
) (EndpointPolicy, error) {
	p := &endpointPolicy{hosts: map[string]struct{}{}, resolver: resolver, dialer: dialer}
	if p.resolver == nil {
		p.resolver = net.DefaultResolver
	}
	if p.dialer == nil {
		p.dialer = (&net.Dialer{}).DialContext
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
	_, _, _, err := p.resolve(ctx, endpoint)
	return err
}

func (p *endpointPolicy) PinnedClient(ctx context.Context, endpoint string, template *http.Client) (*http.Client, error) {
	parsed, host, ip, err := p.resolve(ctx, endpoint)
	if err != nil {
		return nil, err
	}
	port := parsed.Port()
	if port == "" {
		if strings.EqualFold(parsed.Scheme, "https") {
			port = "443"
		} else {
			port = "80"
		}
	}
	targetAddress := net.JoinHostPort(ip.String(), port)

	baseTransport := http.DefaultTransport.(*http.Transport)
	if template != nil && template.Transport != nil {
		var ok bool
		baseTransport, ok = template.Transport.(*http.Transport)
		if !ok {
			return nil, ErrEndpointNotAllowed
		}
	}
	transport := baseTransport.Clone()
	transport.Proxy = nil
	transport.DialTLS = nil
	transport.DialTLSContext = nil
	transport.DialContext = func(dialCtx context.Context, network, address string) (net.Conn, error) {
		addressHost, addressPort, splitErr := net.SplitHostPort(address)
		if splitErr != nil || !strings.EqualFold(strings.Trim(addressHost, "[]"), host) || addressPort != port {
			return nil, ErrEndpointNotAllowed
		}
		return p.dialer(dialCtx, network, targetAddress)
	}
	if strings.EqualFold(parsed.Scheme, "https") {
		if transport.TLSClientConfig == nil {
			transport.TLSClientConfig = &tls.Config{}
		} else {
			transport.TLSClientConfig = transport.TLSClientConfig.Clone()
		}
		transport.TLSClientConfig.ServerName = host
	}
	client := &http.Client{}
	if template != nil {
		*client = *template
	}
	client.Transport = transport
	client.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	return client, nil
}

func (p *endpointPolicy) resolve(ctx context.Context, endpoint string) (*url.URL, string, net.IP, error) {
	parsed, err := url.ParseRequestURI(endpoint)
	if err != nil || parsed.Host == "" || parsed.User != nil {
		return nil, "", nil, ErrEndpointNotAllowed
	}
	scheme := strings.ToLower(parsed.Scheme)
	if scheme != "http" && scheme != "https" {
		return nil, "", nil, ErrEndpointNotAllowed
	}
	host := strings.ToLower(parsed.Hostname())
	if host == "" {
		return nil, "", nil, ErrEndpointNotAllowed
	}
	hostAllowed := false
	if _, ok := p.hosts[host]; ok {
		hostAllowed = true
	}
	var addresses []net.IPAddr
	if ip := net.ParseIP(host); ip != nil {
		addresses = []net.IPAddr{{IP: ip}}
	} else {
		addresses, err = p.resolver.LookupIPAddr(ctx, host)
		if err != nil || len(addresses) == 0 {
			return nil, "", nil, ErrEndpointNotAllowed
		}
	}
	for _, address := range addresses {
		if address.IP == nil || (!hostAllowed && !p.ipAllowed(address.IP)) {
			return nil, "", nil, ErrEndpointNotAllowed
		}
	}
	return parsed, host, addresses[0].IP, nil
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
