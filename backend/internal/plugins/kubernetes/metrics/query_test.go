package metrics

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestBuildRangeQueryUsesNamedTemplateAndSafeParameters(t *testing.T) {
	now := time.Unix(1_700_003_600, 0).UTC()

	query, err := BuildRangeQuery(QueryRequest{
		Template:  "pod_cpu",
		Range:     "1h",
		Namespace: "default",
		Pod:       "api-0",
	}, now)

	require.NoError(t, err)
	require.Equal(t, "/api/v1/query_range", query.Path)
	require.Equal(t, `sum(rate(container_cpu_usage_seconds_total{image!="",pod="api-0",namespace="default"}[1m])) by (pod,namespace)`, query.Values.Get("query"))
	require.Equal(t, "1700000000", query.Values.Get("start"))
	require.Equal(t, "1700003600", query.Values.Get("end"))
	require.Equal(t, "60", query.Values.Get("step"))
}

func TestBuildRangeQueryRequiresTemplateParameters(t *testing.T) {
	_, err := BuildRangeQuery(QueryRequest{
		Template: "node_memory",
		Range:    "15m",
	}, time.Unix(1_700_000_000, 0).UTC())

	require.ErrorIs(t, err, ErrInvalidRequest)
	require.Contains(t, err.Error(), "missing required parameter")
}

func TestBuildRangeQueryRejectsUnsafeParameters(t *testing.T) {
	_, err := BuildRangeQuery(QueryRequest{
		Template:  "pod_memory",
		Range:     "5m",
		Namespace: "default",
		Pod:       `api".*`,
	}, time.Unix(1_700_000_000, 0).UTC())

	require.ErrorIs(t, err, ErrInvalidRequest)
	require.Contains(t, err.Error(), "invalid parameter")
}

func TestBuildRangeQueryRejectsUnsafeUnusedParameters(t *testing.T) {
	_, err := BuildRangeQuery(QueryRequest{
		Template: "node_cpu",
		Range:    "5m",
		Node:     "node-1",
		Pod:      `api".*`,
	}, time.Unix(1_700_000_000, 0).UTC())

	require.ErrorIs(t, err, ErrInvalidRequest)
	require.Contains(t, err.Error(), "invalid parameter")
}

func TestBuildRangeQueryRejectsUnknownTemplateAndRange(t *testing.T) {
	now := time.Unix(1_700_000_000, 0).UTC()

	_, templateErr := BuildRangeQuery(QueryRequest{Template: "raw", Range: "1h"}, now)
	require.ErrorIs(t, templateErr, ErrUnknownTemplate)

	_, rangeErr := BuildRangeQuery(QueryRequest{Template: "node_cpu", Range: "7d", Node: "node-1"}, now)
	require.ErrorIs(t, rangeErr, ErrInvalidRequest)
	require.Contains(t, rangeErr.Error(), "invalid range")
}

func TestBuildRangeQueryParsesRFC3339AndUnixTime(t *testing.T) {
	now := time.Unix(1_700_003_600, 0).UTC()

	rfc3339, err := BuildRangeQuery(QueryRequest{
		Template: "node_cpu",
		Range:    "5m",
		Node:     "node-1.example",
		Time:     "2023-11-14T23:13:20Z",
	}, now)
	require.NoError(t, err)
	require.Equal(t, "1700003300", rfc3339.Values.Get("start"))
	require.Equal(t, "1700003600", rfc3339.Values.Get("end"))

	unix, err := BuildRangeQuery(QueryRequest{
		Template: "node_cpu",
		Range:    "5m",
		Node:     "node-1.example",
		Time:     "1700003600",
	}, now)
	require.NoError(t, err)
	require.Equal(t, "1700003300", unix.Values.Get("start"))
	require.Equal(t, "1700003600", unix.Values.Get("end"))
}

func TestClientRejectsNonPrometheusAPIPaths(t *testing.T) {
	prometheus := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("client must not call non-Prometheus API paths")
	}))
	t.Cleanup(prometheus.Close)

	_, err := NewClient().Query(context.Background(), prometheus.URL, "", "", PrometheusQuery{Path: "/redirect"})

	require.ErrorIs(t, err, ErrInvalidRequest)
}
