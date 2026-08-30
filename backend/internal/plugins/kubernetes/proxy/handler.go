package proxy

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httputil"
	"net/url"
	"regexp"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/authn"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/httpx"
	sparkplugin "github.com/sparkops/sparkops-agent/backend/internal/platform/plugin"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

const maxUpstreamBodyBytes int64 = 16 << 20

var sensitiveKeyRE = regexp.MustCompile(`(?i)(certificate|token|client-key|private-key|password)`)

type Handler struct {
	ctx sparkplugin.PluginContext
}

func NewHandler(ctx sparkplugin.PluginContext) *Handler {
	return &Handler{ctx: ctx}
}

func (h *Handler) ServeGIN(c *gin.Context) {
	if h == nil || h.ctx.Clusters == nil {
		httpx.Fail(c, "PLUGIN_KUBERNETES_UNAVAILABLE", "kubernetes plugin context unavailable", http.StatusServiceUnavailable)
		return
	}
	wsID, ok := httpx.WorkspaceIDFromContext(c)
	if !ok {
		httpx.Fail(c, "TENANT_WORKSPACE_REQUIRED", "workspace required", http.StatusBadRequest)
		return
	}
	if _, ok := authn.MemberIDFromContext(c); !ok {
		httpx.Fail(c, "AUTH_UNAUTHORIZED", "authorization required", http.StatusUnauthorized)
		return
	}
	clusterID, err := kernel.ParseClusterInfoID(c.Param("clusterId"))
	if err != nil {
		httpx.Fail(c, "PLUGIN_CLUSTER_INVALID", "invalid cluster id", http.StatusBadRequest)
		return
	}
	apiPath, err := cleanAPIPath(c.Param("path"))
	if err != nil {
		httpx.Fail(c, "PLUGIN_KUBERNETES_PROXY_PATH_INVALID", "invalid kubernetes api path", http.StatusBadRequest)
		return
	}
	if !isReadMethod(c.Request.Method) {
		if h.ctx.Writer == nil {
			httpx.Fail(c, "PLUGIN_KUBERNETES_UNAVAILABLE", "kubernetes plugin context unavailable", http.StatusServiceUnavailable)
			return
		}
		if _, _, ok := requireProxyWrite(c.Request.Context(), c, h.ctx.Writer); !ok {
			return
		}
	}

	clientset, _, restCfg, _, err := h.ctx.Clusters.ForWorkspace(c.Request.Context(), wsID, clusterID)
	if err != nil {
		handleProxyClusterError(c, err)
		return
	}
	if restCfg == nil || strings.TrimSpace(restCfg.Host) == "" {
		httpx.Fail(c, "PLUGIN_CLUSTER_CREDENTIALS_INCOMPLETE", "cluster credentials are incomplete", http.StatusServiceUnavailable)
		return
	}
	apiPath = CompatibleAPIPath(discoverMinor(c.Request.Context(), clientset), apiPath)
	if shouldAggregate(c.Request.Method, c.Request.URL.Query()) {
		h.aggregateList(c, restCfg, apiPath)
		return
	}
	h.reverseProxy(c, restCfg, apiPath)
}

func (h *Handler) reverseProxy(c *gin.Context, restCfg *rest.Config, apiPath string) {
	transport, target, err := transportAndTarget(restCfg)
	if err != nil {
		httpx.Fail(c, "PLUGIN_CLUSTER_CREDENTIALS_INCOMPLETE", "cluster credentials are incomplete", http.StatusServiceUnavailable)
		return
	}
	proxy := httputil.NewSingleHostReverseProxy(target)
	proxy.Transport = transport
	originalDirector := proxy.Director
	proxy.Director = func(req *http.Request) {
		originalDirector(req)
		req.URL.Scheme = target.Scheme
		req.URL.Host = target.Host
		req.URL.Path = apiPath
		req.URL.RawPath = ""
		req.URL.RawQuery = c.Request.URL.RawQuery
		req.Host = target.Host
		req.Header = sanitizedHeaders(c.Request.Header)
	}
	proxy.ModifyResponse = sanitizeErrorResponse
	proxy.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
		http.Error(w, "upstream request failed", http.StatusBadGateway)
	}
	proxy.ServeHTTP(c.Writer, c.Request)
}

func (h *Handler) aggregateList(c *gin.Context, restCfg *rest.Config, apiPath string) {
	opts, err := parseAggregateOptions(c.Request.URL.Query())
	if err != nil {
		httpx.Fail(c, "PLUGIN_KUBERNETES_PROXY_QUERY_INVALID", err.Error(), http.StatusBadRequest)
		return
	}
	plan, ok := listPlanForPath(apiPath, opts.namespaces)
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
	var template map[string]any
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
		var obj map[string]any
		if err := json.Unmarshal(body, &obj); err != nil {
			h.reverseProxy(c, restCfg, apiPath)
			return
		}
		items, ok := obj["items"].([]any)
		if !ok {
			h.reverseProxy(c, restCfg, apiPath)
			return
		}
		if template == nil {
			template = obj
		}
		merged = append(merged, items...)
	}
	merged = filterByKeyword(merged, opts.keyword)
	total := len(merged)
	if opts.hasPage {
		merged = pageItems(merged, opts.page, opts.pageSize)
		c.JSON(http.StatusOK, gin.H{
			"total":    total,
			"items":    merged,
			"page":     opts.page,
			"pageSize": opts.pageSize,
		})
		return
	}
	if template != nil {
		template["items"] = merged
		template["total"] = total
		c.JSON(http.StatusOK, template)
		return
	}
	c.JSON(http.StatusOK, gin.H{"total": total, "items": merged})
}

func cleanAPIPath(raw string) (string, error) {
	if raw == "" {
		raw = "/"
	}
	if strings.Contains(raw, "\x00") || strings.Contains(raw, `\`) || strings.Contains(raw, "://") || strings.HasPrefix(raw, "//") {
		return "", errors.New("invalid path")
	}
	decoded, err := url.PathUnescape(raw)
	if err != nil {
		return "", err
	}
	if strings.Contains(decoded, "\x00") || strings.Contains(decoded, `\`) || strings.Contains(decoded, "://") || strings.HasPrefix(decoded, "//") {
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

func allowedAPIPath(apiPath string) bool {
	for _, prefix := range []string{"/api/", "/apis/", "/version", "/healthz", "/readyz", "/livez"} {
		if strings.HasPrefix(apiPath, prefix) {
			return true
		}
	}
	return false
}

func isReadMethod(method string) bool {
	return method == http.MethodGet || method == http.MethodHead
}

func shouldAggregate(method string, q url.Values) bool {
	if method != http.MethodGet || strings.EqualFold(q.Get("watch"), "true") {
		return false
	}
	for _, key := range []string{"page", "pageSize", "keyword", "ns"} {
		if _, ok := q[key]; ok {
			return true
		}
	}
	return false
}

type aggregateOptions struct {
	namespaces []string
	keyword    string
	page       int
	pageSize   int
	hasPage    bool
}

func parseAggregateOptions(q url.Values) (aggregateOptions, error) {
	var opts aggregateOptions
	opts.keyword = q.Get("keyword")
	opts.namespaces = splitNamespaces(q["ns"])
	pageRaw, hasPage := q["page"]
	pageSizeRaw, hasPageSize := q["pageSize"]
	opts.hasPage = hasPage || hasPageSize
	if opts.hasPage {
		if !hasPage || !hasPageSize || len(pageRaw) != 1 || len(pageSizeRaw) != 1 {
			return opts, errors.New("page and pageSize are required together")
		}
		page, err := strconv.Atoi(pageRaw[0])
		if err != nil || page <= 0 {
			return opts, errors.New("page must be a positive integer")
		}
		pageSize, err := strconv.Atoi(pageSizeRaw[0])
		if err != nil || pageSize <= 0 || pageSize > 500 {
			return opts, errors.New("pageSize must be a positive integer no greater than 500")
		}
		opts.page = page
		opts.pageSize = pageSize
	}
	return opts, nil
}

func splitNamespaces(values []string) []string {
	var out []string
	for _, value := range values {
		for _, part := range strings.Split(value, ",") {
			ns := strings.TrimSpace(part)
			if ns != "" && !strings.Contains(ns, "/") && !strings.Contains(ns, `\`) && !strings.Contains(ns, "\x00") && ns != ".." {
				out = append(out, ns)
			}
		}
	}
	return out
}

type listPlan struct {
	paths []string
}

func listPlanForPath(apiPath string, namespaces []string) (listPlan, bool) {
	segments := pathSegments(apiPath)
	if len(segments) == 0 {
		return listPlan{}, false
	}
	if idx := namespaceSegmentIndex(segments); idx >= 0 {
		if len(segments) == idx+3 {
			return listPlan{paths: []string{apiPath}}, true
		}
		return listPlan{}, false
	}
	if !isClusterListPath(segments) {
		return listPlan{}, false
	}
	resource := segments[len(segments)-1]
	if isClusterScopedResource(resource) || len(namespaces) == 0 {
		return listPlan{paths: []string{apiPath}}, true
	}
	paths := make([]string, 0, len(namespaces))
	for _, ns := range namespaces {
		paths = append(paths, namespacedListPath(segments, ns))
	}
	return listPlan{paths: paths}, true
}

func pathSegments(apiPath string) []string {
	parts := strings.Split(strings.Trim(apiPath, "/"), "/")
	if len(parts) == 1 && parts[0] == "" {
		return nil
	}
	return parts
}

func namespaceSegmentIndex(segments []string) int {
	for i := 0; i < len(segments); i++ {
		if segments[i] == "namespaces" {
			return i
		}
	}
	return -1
}

func isClusterListPath(segments []string) bool {
	if len(segments) == 3 && segments[0] == "api" {
		return true
	}
	return len(segments) == 4 && segments[0] == "apis"
}

func isClusterScopedResource(resource string) bool {
	switch resource {
	case "apiservices", "clusterrolebindings", "clusterroles", "customresourcedefinitions", "mutatingwebhookconfigurations",
		"namespaces", "nodes", "persistentvolumes", "priorityclasses", "storageclasses", "validatingwebhookconfigurations":
		return true
	default:
		return false
	}
}

func namespacedListPath(segments []string, namespace string) string {
	resource := segments[len(segments)-1]
	prefix := append([]string{}, segments[:len(segments)-1]...)
	prefix = append(prefix, "namespaces", url.PathEscape(namespace), resource)
	return "/" + strings.Join(prefix, "/")
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

func readLimited(r io.Reader, limit int64) ([]byte, error) {
	var buf bytes.Buffer
	n, err := io.Copy(&buf, io.LimitReader(r, limit+1))
	if err != nil {
		return nil, err
	}
	if n > limit {
		return nil, errors.New("upstream response too large")
	}
	return buf.Bytes(), nil
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

func pageItems(items []any, page, pageSize int) []any {
	start := (page - 1) * pageSize
	if start >= len(items) {
		return []any{}
	}
	end := start + pageSize
	if end > len(items) {
		end = len(items)
	}
	return items[start:end]
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

func sanitizeErrorResponse(resp *http.Response) error {
	if resp == nil || resp.StatusCode < http.StatusBadRequest || resp.Body == nil {
		return nil
	}
	body, err := readLimited(resp.Body, maxUpstreamBodyBytes)
	_ = resp.Body.Close()
	if err != nil {
		body = []byte(`{"message":"upstream error body too large"}`)
	}
	body = sanitizeBody(body)
	resp.Body = io.NopCloser(bytes.NewReader(body))
	resp.ContentLength = int64(len(body))
	resp.Header.Set("Content-Length", strconv.Itoa(len(body)))
	return nil
}

func sanitizeBody(body []byte) []byte {
	var v any
	if json.Unmarshal(body, &v) == nil {
		redactJSON(&v)
		if out, err := json.Marshal(v); err == nil {
			return out
		}
	}
	return redactPlaintext(body)
}

func redactJSON(v *any) {
	switch typed := (*v).(type) {
	case map[string]any:
		for key, value := range typed {
			if sensitiveKeyRE.MatchString(key) {
				typed[key] = "redacted"
				continue
			}
			child := value
			redactJSON(&child)
			typed[key] = child
		}
	case []any:
		for i, value := range typed {
			child := value
			redactJSON(&child)
			typed[i] = child
		}
	case string:
		if sensitiveKeyRE.MatchString(typed) {
			*v = "redacted"
		}
	}
}

func redactPlaintext(body []byte) []byte {
	lines := strings.Split(string(body), "\n")
	for i, line := range lines {
		lower := strings.ToLower(line)
		for _, key := range []string{"certificate", "token", "client-key", "private-key", "password"} {
			if idx := strings.Index(lower, key); idx >= 0 {
				lines[i] = line[:idx+len(key)] + "=redacted"
				break
			}
		}
	}
	return []byte(strings.Join(lines, "\n"))
}

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

func discoverMinor(ctx context.Context, clientset kubernetes.Interface) int {
	if clientset == nil {
		return 0
	}
	version, err := clientset.Discovery().ServerVersion()
	if err != nil || version == nil {
		return 0
	}
	digits := strings.Builder{}
	for _, r := range version.Minor {
		if r < '0' || r > '9' {
			break
		}
		digits.WriteRune(r)
	}
	if digits.Len() == 0 {
		return 0
	}
	minor, err := strconv.Atoi(digits.String())
	if err != nil {
		return 0
	}
	return minor
}

func requireProxyWrite(ctx context.Context, c *gin.Context, writer sparkplugin.WriteAuthorizer) (kernel.WorkspaceID, kernel.MemberID, bool) {
	wsID, ok := httpx.WorkspaceIDFromContext(c)
	if !ok {
		httpx.Fail(c, "TENANT_WORKSPACE_REQUIRED", "workspace required", http.StatusBadRequest)
		return kernel.WorkspaceID{}, kernel.MemberID{}, false
	}
	memberID, ok := authn.MemberIDFromContext(c)
	if !ok {
		httpx.Fail(c, "AUTH_UNAUTHORIZED", "authorization required", http.StatusUnauthorized)
		return kernel.WorkspaceID{}, kernel.MemberID{}, false
	}
	allowed, err := writer.CanWrite(ctx, wsID, memberID)
	if err != nil {
		httpx.HandleError(c, err)
		return kernel.WorkspaceID{}, kernel.MemberID{}, false
	}
	if !allowed {
		httpx.Fail(c, "GOVERNANCE_API_ACCESS_DENIED", "governance admin permission required", http.StatusForbidden)
		return kernel.WorkspaceID{}, kernel.MemberID{}, false
	}
	return wsID, memberID, true
}

func handleProxyClusterError(c *gin.Context, err error) {
	if de, ok := kernel.IsDomainError(err); ok {
		switch de.Code() {
		case "PLUGIN_CLUSTER_FORBIDDEN":
			httpx.Fail(c, de.Code(), "cluster is not granted to workspace", http.StatusForbidden)
			return
		case "PLUGIN_CLUSTER_CREDENTIALS_INCOMPLETE":
			httpx.Fail(c, de.Code(), "cluster credentials are incomplete", http.StatusServiceUnavailable)
			return
		}
	}
	httpx.Fail(c, "PLUGIN_KUBERNETES_PROXY_UPSTREAM_UNAVAILABLE", "cluster credentials are unavailable", http.StatusBadGateway)
}

func copyHeaders(dst, src http.Header) {
	for key, values := range src {
		dst.Del(key)
		for _, value := range values {
			dst.Add(key, value)
		}
	}
}
