package metrics

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	sparkplugin "github.com/sparkops/sparkops-agent/backend/internal/platform/plugin"
)

const (
	queryPath      = "/api/v1/query"
	queryRangePath = "/api/v1/query_range"
	maxBodyBytes   = 8 << 20
)

var (
	ErrInvalidRequest  = errors.New("invalid metrics request")
	ErrUnknownTemplate = errors.New("unknown metrics template")
	ErrUpstream        = errors.New("prometheus upstream request failed")
)

var safeParameter = regexp.MustCompile(`^[a-z0-9.-]+$`)

type QueryRequest struct {
	Template  string `json:"template"`
	Range     string `json:"range"`
	Node      string `json:"node"`
	Pod       string `json:"pod"`
	Namespace string `json:"namespace"`
	Time      string `json:"time"`
}

type PrometheusQuery struct {
	Path   string
	Values url.Values
}

type Client struct {
	httpClient *http.Client
	policy     sparkplugin.EndpointPolicy
}

type templateSpec struct {
	query    string
	required []string
}

var templates = map[string]templateSpec{
	"node_cpu": {
		query:    `sum(rate(node_cpu_seconds_total{mode=~"user|system",instance="__node__"}[1m])) by (instance)`,
		required: []string{"node"},
	},
	"node_memory": {
		query:    `sum(node_memory_MemTotal_bytes{instance="__node__"} - (node_memory_MemFree_bytes{instance="__node__"} + node_memory_Buffers_bytes{instance="__node__"} + node_memory_Cached_bytes{instance="__node__"})) by (instance)`,
		required: []string{"node"},
	},
	"pod_cpu": {
		query:    `sum(rate(container_cpu_usage_seconds_total{image!="",pod="__pod__",namespace="__namespace__"}[1m])) by (pod,namespace)`,
		required: []string{"namespace", "pod"},
	},
	"pod_memory": {
		query:    `sum(container_memory_working_set_bytes{image!="",pod="__pod__",namespace="__namespace__"}) by (pod,namespace)`,
		required: []string{"namespace", "pod"},
	},
}

var allowedRanges = map[string]time.Duration{
	"5m":  5 * time.Minute,
	"15m": 15 * time.Minute,
	"1h":  time.Hour,
	"6h":  6 * time.Hour,
	"24h": 24 * time.Hour,
}

func BuildRangeQuery(req QueryRequest, now time.Time) (PrometheusQuery, error) {
	spec, ok := templates[req.Template]
	if !ok {
		return PrometheusQuery{}, ErrUnknownTemplate
	}
	duration, ok := allowedRanges[req.Range]
	if !ok {
		return PrometheusQuery{}, fmt.Errorf("%w: invalid range", ErrInvalidRequest)
	}
	params := map[string]string{
		"node":      req.Node,
		"pod":       req.Pod,
		"namespace": req.Namespace,
	}
	for _, value := range params {
		if value != "" && !safeParameter.MatchString(value) {
			return PrometheusQuery{}, fmt.Errorf("%w: invalid parameter", ErrInvalidRequest)
		}
	}
	for _, name := range spec.required {
		value := params[name]
		if value == "" {
			return PrometheusQuery{}, fmt.Errorf("%w: missing required parameter", ErrInvalidRequest)
		}
	}
	end, err := parseEndTime(req.Time, now)
	if err != nil {
		return PrometheusQuery{}, err
	}
	promql := spec.query
	promql = strings.ReplaceAll(promql, "__node__", req.Node)
	promql = strings.ReplaceAll(promql, "__pod__", req.Pod)
	promql = strings.ReplaceAll(promql, "__namespace__", req.Namespace)
	values := url.Values{}
	values.Set("query", promql)
	values.Set("start", strconv.FormatInt(end.Add(-duration).Unix(), 10))
	values.Set("end", strconv.FormatInt(end.Unix(), 10))
	values.Set("step", "60")
	return PrometheusQuery{Path: queryRangePath, Values: values}, nil
}

func NewClient(policies ...sparkplugin.EndpointPolicy) *Client {
	var policy sparkplugin.EndpointPolicy
	if len(policies) > 0 {
		policy = policies[0]
	}
	return &Client{policy: policy, httpClient: &http.Client{
		Timeout: 10 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}}
}

func (c *Client) Query(ctx context.Context, baseURL, user, password string, query PrometheusQuery) (json.RawMessage, error) {
	if c == nil || c.httpClient == nil {
		c = NewClient()
	}
	if query.Path != queryPath && query.Path != queryRangePath {
		return nil, fmt.Errorf("%w: invalid prometheus path", ErrInvalidRequest)
	}
	endpoint, err := buildEndpoint(baseURL, query.Path, query.Values)
	if err != nil {
		return nil, fmt.Errorf("%w: invalid base url", ErrInvalidRequest)
	}
	if c.policy == nil {
		return nil, ErrUpstream
	}
	httpClient, err := c.policy.PinnedClient(ctx, endpoint, c.httpClient)
	if err != nil {
		return nil, ErrUpstream
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("%w: invalid upstream request", ErrInvalidRequest)
	}
	if user != "" || password != "" {
		req.SetBasicAuth(user, password)
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, ErrUpstream
	}
	defer resp.Body.Close()
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return nil, ErrUpstream
	}
	contentType := resp.Header.Get("Content-Type")
	mediaType, _, err := mime.ParseMediaType(contentType)
	if err != nil || (mediaType != "application/json" && !strings.HasSuffix(mediaType, "+json")) {
		return nil, ErrUpstream
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxBodyBytes+1))
	if err != nil || len(body) > maxBodyBytes {
		return nil, ErrUpstream
	}
	if err := validatePrometheusJSON(body); err != nil {
		return nil, err
	}
	return json.RawMessage(body), nil
}

func parseEndTime(raw string, now time.Time) (time.Time, error) {
	if raw == "" {
		return now.UTC(), nil
	}
	if parsed, err := time.Parse(time.RFC3339, raw); err == nil {
		return parsed.UTC(), nil
	}
	seconds, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return time.Time{}, fmt.Errorf("%w: invalid time", ErrInvalidRequest)
	}
	return time.Unix(seconds, 0).UTC(), nil
}

func buildEndpoint(baseURL, path string, values url.Values) (string, error) {
	parsed, err := url.Parse(baseURL)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" || parsed.User != nil {
		return "", err
	}
	scheme := strings.ToLower(parsed.Scheme)
	if scheme != "http" && scheme != "https" {
		return "", errors.New("invalid scheme")
	}
	parsed.Path = strings.TrimRight(parsed.Path, "/") + path
	parsed.RawPath = ""
	parsed.RawQuery = values.Encode()
	parsed.Fragment = ""
	parsed.User = nil
	return parsed.String(), nil
}

func validatePrometheusJSON(body []byte) error {
	var response struct {
		Status string `json:"status"`
		Data   struct {
			ResultType string `json:"resultType"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &response); err != nil {
		return ErrUpstream
	}
	if response.Status != "success" {
		return ErrUpstream
	}
	if response.Data.ResultType != "vector" && response.Data.ResultType != "matrix" {
		return ErrUpstream
	}
	return nil
}
