package proxy

import (
	"errors"
	"net/http"
	"net/url"

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
		canonical := http.CanonicalHeaderKey(key)
		if !allowedForwardHeader(canonical) {
			continue
		}
		out[canonical] = append([]string(nil), values...)
	}
	return out
}

func allowedForwardHeader(key string) bool {
	switch key {
	case "Accept", "Content-Type", "If-Match", "If-None-Match", "If-Modified-Since", "User-Agent":
		return true
	default:
		return false
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
