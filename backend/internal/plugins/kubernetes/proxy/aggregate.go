package proxy

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"math"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/httpx"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
	"k8s.io/apimachinery/pkg/util/validation"
	"k8s.io/client-go/rest"
)

type aggregateOptions struct {
	namespaces []string
	keyword    string
	page       uint64
	pageSize   uint64
	hasPage    bool
}

type listPlan struct {
	paths        []string
	kind         string
	apiVersion   string
	resourcePath apiListPath
}

func (h *Handler) aggregateList(c *gin.Context, clusterID kernel.ClusterInfoID, restCfg *rest.Config, apiPath string) {
	opts, err := parseAggregateOptions(c.Request.URL.Query())
	if err != nil {
		httpx.Fail(c, "PLUGIN_KUBERNETES_PROXY_QUERY_INVALID", err.Error(), http.StatusBadRequest)
		return
	}
	plan, ok, err := h.listPlanForPath(clusterID, restCfg, apiPath, opts.namespaces)
	if err != nil {
		httpx.Fail(c, "PLUGIN_KUBERNETES_PROXY_DISCOVERY_FAILED", "kubernetes api resource discovery failed", http.StatusBadGateway)
		return
	}
	if !ok {
		h.reverseProxy(c, restCfg, apiPath)
		return
	}
	transport, target, err := transportAndTarget(restCfg)
	if err != nil {
		httpx.Fail(c, "PLUGIN_CLUSTER_CREDENTIALS_INCOMPLETE", "cluster credentials are incomplete", http.StatusServiceUnavailable)
		return
	}
	client := &http.Client{Transport: transport}
	cleanQuery := queryWithoutLocalParams(c.Request.URL.Query())
	var merged []any
	for _, reqPath := range plan.paths {
		body, status, header, err := fetchList(c.Request.Context(), client, target, reqPath, cleanQuery, c.Request.Header)
		if err != nil {
			http.Error(c.Writer, "upstream request failed", http.StatusBadGateway)
			return
		}
		if status >= http.StatusBadRequest {
			copyHeaders(c.Writer.Header(), header)
			c.Writer.WriteHeader(status)
			_, _ = c.Writer.Write(sanitizeBody(body))
			return
		}
		obj, items, ok := parseListBody(body)
		if !ok {
			h.reverseProxy(c, restCfg, apiPath)
			return
		}
		if plan.kind == "" {
			plan.kind, _ = obj["kind"].(string)
			plan.apiVersion, _ = obj["apiVersion"].(string)
		}
		merged = append(merged, items...)
	}
	merged = filterByKeyword(merged, opts.keyword)
	total := len(merged)
	if opts.hasPage {
		merged = pageItems(merged, opts.page, opts.pageSize)
	}
	c.JSON(http.StatusOK, aggregateEnvelope(plan.kind, plan.apiVersion, total, merged, opts))
}

func (h *Handler) listPlanForPath(clusterID kernel.ClusterInfoID, restCfg *rest.Config, apiPath string, namespaces []string) (listPlan, bool, error) {
	info, ok := parseListPath(apiPath)
	if !ok {
		return listPlan{}, false, nil
	}
	if info.namespacedInPath || len(namespaces) == 0 {
		return listPlan{paths: []string{apiPath}, resourcePath: info}, true, nil
	}
	namespaced, err := h.discover.resourceNamespaced(clusterID, restCfg, info.groupVersion, info.resource)
	if err != nil {
		return listPlan{}, false, err
	}
	if !namespaced {
		return listPlan{paths: []string{apiPath}, resourcePath: info}, true, nil
	}
	paths := make([]string, 0, len(namespaces))
	for _, ns := range namespaces {
		paths = append(paths, namespacedListPath(info, ns))
	}
	return listPlan{paths: paths, resourcePath: info}, true, nil
}

func parseAggregateOptions(q url.Values) (aggregateOptions, error) {
	var opts aggregateOptions
	for _, key := range []string{"limit", "continue", "resourceVersion", "resourceVersionMatch"} {
		if _, ok := q[key]; ok {
			return opts, errors.New("limit, continue, resourceVersion, and resourceVersionMatch are not supported with aggregation")
		}
	}
	opts.keyword = q.Get("keyword")
	namespaces, err := splitNamespaces(q["ns"])
	if err != nil {
		return opts, err
	}
	opts.namespaces = namespaces
	pageRaw, hasPage := q["page"]
	pageSizeRaw, hasPageSize := q["pageSize"]
	opts.hasPage = hasPage || hasPageSize
	if opts.hasPage {
		if !hasPage || !hasPageSize || len(pageRaw) != 1 || len(pageSizeRaw) != 1 {
			return opts, errors.New("page and pageSize are required together")
		}
		page, err := strconv.ParseUint(pageRaw[0], 10, 64)
		if err != nil || page == 0 || page > 1_000_000 {
			return opts, errors.New("page must be between 1 and 1000000")
		}
		pageSize, err := strconv.ParseUint(pageSizeRaw[0], 10, 64)
		if err != nil || pageSize == 0 || pageSize > 500 {
			return opts, errors.New("pageSize must be between 1 and 500")
		}
		opts.page = page
		opts.pageSize = pageSize
	}
	return opts, nil
}

func splitNamespaces(values []string) ([]string, error) {
	if len(values) == 0 {
		return nil, nil
	}
	var out []string
	seen := map[string]struct{}{}
	for _, value := range values {
		for _, part := range strings.Split(value, ",") {
			ns := strings.TrimSpace(part)
			if ns == "" {
				return nil, errors.New("namespace must be a valid DNS1123 label")
			}
			if errs := validation.IsDNS1123Label(ns); len(errs) > 0 {
				return nil, errors.New("namespace must be a valid DNS1123 label")
			}
			if _, ok := seen[ns]; ok {
				continue
			}
			seen[ns] = struct{}{}
			out = append(out, ns)
		}
	}
	return out, nil
}

func queryWithoutLocalParams(q url.Values) url.Values {
	clean := make(url.Values, len(q))
	for key, values := range q {
		switch key {
		case "page", "pageSize", "keyword", "ns":
			continue
		default:
			clean[key] = append([]string(nil), values...)
		}
	}
	return clean
}

func fetchList(ctx context.Context, client *http.Client, target *url.URL, apiPath string, q url.Values, in http.Header) ([]byte, int, http.Header, error) {
	u := *target
	u.Path = apiPath
	u.RawPath = ""
	u.RawQuery = q.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, 0, nil, err
	}
	req.Host = target.Host
	req.Header = sanitizedHeaders(in)
	resp, err := client.Do(req)
	if err != nil {
		return nil, 0, nil, err
	}
	defer resp.Body.Close()
	body, err := readLimited(resp.Body, maxUpstreamBodyBytes)
	if err != nil {
		return nil, 0, nil, err
	}
	return body, resp.StatusCode, resp.Header.Clone(), nil
}

func parseListBody(body []byte) (map[string]any, []any, bool) {
	var obj map[string]any
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.UseNumber()
	if err := decoder.Decode(&obj); err != nil {
		return nil, nil, false
	}
	items, ok := obj["items"].([]any)
	return obj, items, ok
}

func aggregateEnvelope(kind, apiVersion string, total int, items []any, opts aggregateOptions) gin.H {
	env := gin.H{
		"kind":       kind,
		"apiVersion": apiVersion,
		"metadata":   gin.H{},
		"total":      total,
		"items":      items,
	}
	if opts.hasPage {
		env["page"] = opts.page
		env["pageSize"] = opts.pageSize
	}
	return env
}

func filterByKeyword(items []any, keyword string) []any {
	if keyword == "" {
		return items
	}
	out := make([]any, 0, len(items))
	for _, item := range items {
		if strings.Contains(metadataName(item), keyword) {
			out = append(out, item)
		}
	}
	return out
}

func metadataName(item any) string {
	obj, ok := item.(map[string]any)
	if !ok {
		return ""
	}
	meta, ok := obj["metadata"].(map[string]any)
	if !ok {
		return ""
	}
	name, _ := meta["name"].(string)
	return name
}

func pageItems(items []any, page, pageSize uint64) []any {
	start := (page - 1) * pageSize
	if start > uint64(math.MaxInt) || start >= uint64(len(items)) {
		return []any{}
	}
	end := start + pageSize
	if end > uint64(len(items)) {
		end = uint64(len(items))
	}
	return items[int(start):int(end)]
}
