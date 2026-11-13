package filesystemgateway

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
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
	if _, err := s.auth.VerifyUpload(token, sessionID); err != nil {
		s.failCapability(c, err)
		return
	}

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

	limit := grant.ExpectedSize + 1
	hasher := sha256.New()
	var buf bytes.Buffer
	written, err := io.Copy(io.MultiWriter(&buf, hasher), io.LimitReader(c.Request.Body, limit))
	if err != nil {
		httpx.Fail(c, "UPLOAD_FAILED", "failed to read upload body", http.StatusBadRequest)
		return
	}
	if written > grant.ExpectedSize {
		httpx.Fail(c, "PAYLOAD_TOO_LARGE", ErrPayloadTooLarge.Error(), http.StatusRequestEntityTooLarge)
		return
	}
	checksum := hex.EncodeToString(hasher.Sum(nil))

	_, err = grant.Provider.Put(c.Request.Context(), storage.PutRequest{
		ObjectKey: grant.TemporaryObjectKey,
		Body:      bytes.NewReader(buf.Bytes()),
		Size:      written,
		SHA256:    checksum,
	})
	if err != nil {
		httpx.Fail(c, "UPLOAD_FAILED", fmt.Sprintf("provider put failed: %v", err), http.StatusBadRequest)
		return
	}

	httpx.OK(c, gin.H{"size": written, "sha256": checksum})
}
