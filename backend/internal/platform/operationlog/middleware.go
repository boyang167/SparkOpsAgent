package operationlog

import (
	"bytes"
	"context"
	"io"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/sparkops/sparkops-agent/backend/internal/platform/authn"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/httpx"
)

const maxBodyBytes = 4096

type responseCapture struct {
	gin.ResponseWriter
	body *bytes.Buffer
}

func (w *responseCapture) Write(b []byte) (int, error) {
	if !isEventStreamContentType(w.Header().Get("Content-Type")) && w.body.Len() < maxBodyBytes {
		remain := maxBodyBytes - w.body.Len()
		if len(b) > remain {
			_, _ = w.body.Write(b[:remain])
		} else {
			_, _ = w.body.Write(b)
		}
	}
	return w.ResponseWriter.Write(b)
}

func Middleware(store *Store) gin.HandlerFunc {
	if store == nil {
		return func(c *gin.Context) { c.Next() }
	}
	return func(c *gin.Context) {
		path := c.Request.URL.Path
		if shouldSkip(path) {
			c.Next()
			return
		}

		start := time.Now()
		var reqBody string
		if c.Request.Body != nil && c.Request.Method != "GET" && c.Request.Method != "HEAD" {
			contentType := strings.ToLower(c.GetHeader("Content-Type"))
			if strings.HasPrefix(contentType, "multipart/") {
				// Never truncate multipart: replacing Body with a 4KiB prefix
				// breaks plugin ZIP uploads (unexpected EOF / hung proxies).
				reqBody = "[multipart omitted]"
			} else {
				raw, _ := io.ReadAll(io.LimitReader(c.Request.Body, maxBodyBytes))
				reqBody = string(raw)
				c.Request.Body = io.NopCloser(bytes.NewBuffer(raw))
			}
		}

		capture := &responseCapture{ResponseWriter: c.Writer, body: bytes.NewBuffer(nil)}
		c.Writer = capture
		c.Next()

		memberID := ""
		if id, ok := authn.MemberIDFromContext(c); ok {
			memberID = id.String()
		}
		wsID := extractWorkspaceID(c)
		errMsg := ""
		if len(c.Errors) > 0 {
			errMsg = c.Errors.String()
		}
		respBody := capture.body.String()
		if isEventStreamContentType(c.Writer.Header().Get("Content-Type")) {
			respBody = "[stream omitted]"
		}

		rec := Record{
			IP:           c.ClientIP(),
			Method:       c.Request.Method,
			Path:         path,
			Status:       c.Writer.Status(),
			LatencyMs:    time.Since(start).Milliseconds(),
			Agent:        c.Request.UserAgent(),
			ErrorMessage: errMsg,
			Body:         reqBody,
			Resp:         respBody,
			MemberID:     memberID,
			WorkspaceID:  wsID,
			TraceID:      httpx.GetTraceID(c),
		}
		go func() {
			_ = store.Create(context.Background(), rec)
		}()
	}
}

func isEventStreamContentType(contentType string) bool {
	return strings.HasPrefix(strings.ToLower(strings.TrimSpace(contentType)), "text/event-stream")
}

func shouldSkip(path string) bool {
	switch {
	case path == "/health", path == "/metrics", path == "/ready":
		return true
	case strings.HasPrefix(path, "/swagger"), strings.HasPrefix(path, "/docs"):
		return true
	default:
		return false
	}
}

func extractWorkspaceID(c *gin.Context) string {
	if wsID := c.Param("wsId"); wsID != "" {
		return wsID
	}
	if id, ok := httpx.WorkspaceIDFromContext(c); ok {
		return id.String()
	}
	if raw := strings.TrimSpace(c.GetHeader("X-Workspace-Id")); raw != "" {
		return raw
	}
	path := c.Request.URL.Path
	const prefix = "/api/v1/workspaces/"
	if !strings.HasPrefix(path, prefix) {
		return ""
	}
	rest := strings.TrimPrefix(path, prefix)
	if idx := strings.Index(rest, "/"); idx > 0 {
		return rest[:idx]
	}
	return ""
}
