package filesystemgateway

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/httpx"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/storage"
)

func (s *Server) handleDownload(c *gin.Context) {
	fileID := c.Param("fileId")
	token, ok := bearerToken(c.GetHeader("Authorization"))
	if !ok {
		httpx.Fail(c, "UNAUTHORIZED", "missing capability token", http.StatusUnauthorized)
		return
	}
	if _, err := s.auth.VerifyDownload(token, fileID); err != nil {
		s.failCapability(c, err)
		return
	}

	grant, err := s.control.ResolveDownload(c.Request.Context(), fileID)
	if err != nil {
		httpx.Fail(c, "GRANT_NOT_FOUND", "download grant unavailable", http.StatusNotFound)
		return
	}
	if grant.Provider == nil || grant.ObjectKey == "" {
		httpx.Fail(c, "GRANT_INVALID", "download grant incomplete", http.StatusInternalServerError)
		return
	}
	if grant.Classification == model.TransferClassificationRestricted && !grant.AuditOK {
		c.Status(http.StatusForbidden)
		return
	}

	offset, length, status := parseRangeHeader(c.GetHeader("Range"))
	if status == http.StatusRequestedRangeNotSatisfiable {
		c.Status(status)
		return
	}
	if offset > 0 || length > 0 {
		if caps := grant.Provider.Capabilities(); !caps.RangeRead {
			httpx.Fail(c, "RANGE_NOT_SUPPORTED", "range downloads not supported", http.StatusRequestedRangeNotSatisfiable)
			return
		}
	}

	reader, info, err := grant.Provider.Get(c.Request.Context(), storage.GetRequest{
		ObjectKey: grant.ObjectKey,
		Offset:    offset,
		Length:    length,
	})
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			httpx.Fail(c, "NOT_FOUND", "object not found", http.StatusNotFound)
			return
		}
		httpx.Fail(c, "DOWNLOAD_FAILED", fmt.Sprintf("provider get failed: %v", err), http.StatusBadRequest)
		return
	}
	defer reader.Close()

	if status == http.StatusPartialContent {
		c.Header("Content-Range", fmt.Sprintf("bytes %d-%d/%d", offset, offset+contentLength(offset, length, info.Size)-1, info.Size))
	}
	if info.SHA256 != "" {
		c.Header("X-Content-SHA256", info.SHA256)
	}
	c.Header("Content-Length", strconv.FormatInt(contentLength(offset, length, info.Size), 10))
	c.Header("Content-Type", "application/octet-stream")
	c.Status(status)
	_, _ = io.Copy(c.Writer, reader)
}

func parseRangeHeader(value string) (offset, length int64, status int) {
	status = http.StatusOK
	value = strings.TrimSpace(value)
	if value == "" {
		return 0, 0, status
	}
	if !strings.HasPrefix(value, "bytes=") {
		return 0, 0, http.StatusRequestedRangeNotSatisfiable
	}
	spec := strings.TrimPrefix(value, "bytes=")
	parts := strings.Split(spec, "-")
	if len(parts) != 2 {
		return 0, 0, http.StatusRequestedRangeNotSatisfiable
	}
	start, err := strconv.ParseInt(strings.TrimSpace(parts[0]), 10, 64)
	if err != nil || start < 0 {
		return 0, 0, http.StatusRequestedRangeNotSatisfiable
	}
	offset = start
	status = http.StatusPartialContent
	if parts[1] == "" {
		return offset, 0, status
	}
	end, err := strconv.ParseInt(strings.TrimSpace(parts[1]), 10, 64)
	if err != nil || end < start {
		return 0, 0, http.StatusRequestedRangeNotSatisfiable
	}
	length = end - start + 1
	return offset, length, status
}

func contentLength(offset, length, total int64) int64 {
	if length > 0 {
		return length
	}
	if total > offset {
		return total - offset
	}
	return 0
}
