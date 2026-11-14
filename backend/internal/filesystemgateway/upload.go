package filesystemgateway

import (
	"fmt"
	"io"
	"log"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/sparkops/sparkops-agent/backend/internal/platform/httpx"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/storage"
)

func (s *Server) handleUpload(c *gin.Context) {
	sessionID := c.Param("sessionId")
	token, ok := bearerToken(c.GetHeader("Authorization"))
	if !ok {
		httpx.Fail(c, "UNAUTHORIZED", "missing capability token", http.StatusUnauthorized)
		return
	}
	claims, err := s.auth.VerifyUpload(token, sessionID)
	if err != nil {
		s.failCapability(c, err)
		return
	}
	completed := false
	defer func() {
		if !completed {
			s.auth.Release(claims)
		}
	}()

	grant, err := s.control.ResolveUpload(c.Request.Context(), sessionID)
	if err != nil {
		httpx.Fail(c, "GRANT_NOT_FOUND", "upload grant unavailable", http.StatusNotFound)
		return
	}
	if grant.Provider == nil || grant.TemporaryObjectKey == "" {
		httpx.Fail(c, "GRANT_INVALID", "upload grant incomplete", http.StatusInternalServerError)
		return
	}
	if grant.ExpectedSize < 0 || grant.ExpectedSize > s.maxFileBytes {
		httpx.Fail(c, "PAYLOAD_TOO_LARGE", "declared transfer size exceeds limit", http.StatusRequestEntityTooLarge)
		return
	}
	if c.Request.ContentLength > grant.ExpectedSize {
		httpx.Fail(c, "PAYLOAD_TOO_LARGE", ErrPayloadTooLarge.Error(), http.StatusRequestEntityTooLarge)
		return
	}

	body := &countingReader{reader: io.LimitReader(c.Request.Body, grant.ExpectedSize+1)}
	info, err := grant.Provider.Put(c.Request.Context(), storage.PutRequest{
		ObjectKey: grant.TemporaryObjectKey,
		Body:      body,
		Size:      grant.ExpectedSize,
	})
	if err != nil {
		if body.count > grant.ExpectedSize {
			httpx.Fail(c, "PAYLOAD_TOO_LARGE", ErrPayloadTooLarge.Error(), http.StatusRequestEntityTooLarge)
			return
		}
		httpx.Fail(c, "UPLOAD_FAILED", fmt.Sprintf("provider put failed: %v", err), http.StatusBadRequest)
		return
	}

	if err := s.control.RecordUploaded(c.Request.Context(), sessionID, info.Size, info.SHA256); err != nil {
		log.Printf("record uploaded session %s: %v", sessionID, err)
		httpx.Fail(c, "UPLOAD_NOTIFY_FAILED", "control plane upload notification failed", http.StatusBadGateway)
		return
	}
	completed = true

	httpx.OK(c, gin.H{"size": info.Size, "sha256": info.SHA256})
}

type countingReader struct {
	reader io.Reader
	count  int64
}

func (r *countingReader) Read(p []byte) (int, error) {
	n, err := r.reader.Read(p)
	r.count += int64(n)
	return n, err
}
