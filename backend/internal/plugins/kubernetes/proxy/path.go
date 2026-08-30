package proxy

import (
	"errors"
	"net/http"
	"net/url"
	"strings"
)

func cleanAPIPath(raw string) (string, error) {
	if raw == "" {
		raw = "/"
	}
	if hasUnsafePathBytes(raw) || strings.Contains(raw, "://") || strings.HasPrefix(raw, "//") {
		return "", errors.New("invalid path")
	}
	decoded, err := url.PathUnescape(raw)
	if err != nil {
		return "", err
	}
	if hasUnsafePathBytes(decoded) || strings.Contains(decoded, "://") || strings.HasPrefix(decoded, "//") {
		return "", errors.New("invalid path")
	}
	if !strings.HasPrefix(decoded, "/") {
		decoded = "/" + decoded
	}
	for _, part := range strings.Split(decoded, "/") {
		if part == ".." {
			return "", errors.New("invalid path")
		}
	}
	if !allowedAPIPath(decoded) {
		return "", errors.New("invalid path")
	}
	return decoded, nil
}

func hasUnsafePathBytes(s string) bool {
	return strings.Contains(s, "\x00") || strings.Contains(s, `\`)
}

func allowedAPIPath(apiPath string) bool {
	for _, prefix := range []string{"/api/", "/apis/"} {
		if strings.HasPrefix(apiPath, prefix) {
			return true
		}
	}
	for _, root := range []string{"/version", "/healthz", "/readyz", "/livez"} {
		if apiPath == root || strings.HasPrefix(apiPath, root+"/") {
			return true
		}
	}
	return false
}

func shouldAggregate(method string, q url.Values) bool {
	if method != http.MethodGet || isWatch(q) {
		return false
	}
	for _, key := range []string{"page", "pageSize", "keyword", "ns"} {
		if _, ok := q[key]; ok {
			return true
		}
	}
	return false
}

func isWatch(q url.Values) bool {
	return strings.EqualFold(q.Get("watch"), "true")
}

type apiListPath struct {
	groupVersion      string
	resource          string
	namespacedInPath  bool
	segments          []string
	resourceSegOffset int
}

func parseListPath(apiPath string) (apiListPath, bool) {
	segments := pathSegments(apiPath)
	if len(segments) == 3 && segments[0] == "api" {
		return apiListPath{groupVersion: segments[1], resource: segments[2], segments: segments, resourceSegOffset: 2}, true
	}
	if len(segments) == 5 && segments[0] == "api" && segments[2] == "namespaces" {
		return apiListPath{groupVersion: segments[1], resource: segments[4], namespacedInPath: true, segments: segments, resourceSegOffset: 4}, true
	}
	if len(segments) == 4 && segments[0] == "apis" {
		return apiListPath{groupVersion: segments[1] + "/" + segments[2], resource: segments[3], segments: segments, resourceSegOffset: 3}, true
	}
	if len(segments) == 6 && segments[0] == "apis" && segments[3] == "namespaces" {
		return apiListPath{groupVersion: segments[1] + "/" + segments[2], resource: segments[5], namespacedInPath: true, segments: segments, resourceSegOffset: 5}, true
	}
	return apiListPath{}, false
}

func pathSegments(apiPath string) []string {
	parts := strings.Split(strings.Trim(apiPath, "/"), "/")
	if len(parts) == 1 && parts[0] == "" {
		return nil
	}
	return parts
}

func joinPathSegments(segments []string) string {
	return strings.Join(segments, "/")
}

func namespacedListPath(info apiListPath, namespace string) string {
	segments := append([]string{}, info.segments[:info.resourceSegOffset]...)
	segments = append(segments, "namespaces", url.PathEscape(namespace), info.resource)
	return "/" + joinPathSegments(segments)
}
