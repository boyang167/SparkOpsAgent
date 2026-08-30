package proxy

import (
	"errors"
	"net/http"
	"net/url"
	"strings"

	"k8s.io/client-go/rest"
)

func transportAndTarget(restCfg *rest.Config) (http.RoundTripper, *url.URL, error) {
	transport, err := rest.TransportFor(restCfg)
	if err != nil {
		return nil, nil, err
	}
	target, err := url.Parse(restCfg.Host)
	if err != nil || target.Scheme == "" || target.Host == "" {
		return nil, nil, errors.New("invalid target host")
	}
	return transport, target, nil
}

func sanitizedHeaders(in http.Header) http.Header {
	out := make(http.Header, len(in))
	for key, values := range in {
		if shouldDropHeader(key) {
			continue
		}
		out[key] = append([]string(nil), values...)
	}
	removeHopByHop(out)
	return out
}

func shouldDropHeader(key string) bool {
	switch http.CanonicalHeaderKey(key) {
	case "Authorization", "Cookie", "X-Workspace-Id":
		return true
	default:
		return false
	}
}

func removeHopByHop(header http.Header) {
	for _, connectionValue := range header.Values("Connection") {
		for _, token := range strings.Split(connectionValue, ",") {
			header.Del(strings.TrimSpace(token))
		}
	}
	for _, key := range []string{"Connection", "Keep-Alive", "Proxy-Authenticate", "Proxy-Authorization", "Te", "Trailer", "Transfer-Encoding", "Upgrade"} {
		header.Del(key)
	}
}

func copyHeaders(dst, src http.Header) {
	for key, values := range src {
		dst.Del(key)
		for _, value := range values {
			dst.Add(key, value)
		}
	}
}
