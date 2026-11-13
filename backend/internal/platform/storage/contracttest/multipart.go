package contracttest

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/sparkops/sparkops-agent/backend/internal/platform/storage"
)

func RunMultipart(t *testing.T, mp storage.MultipartProvider, base storage.Provider) {
	t.Helper()
	ctx := context.Background()
	caps := base.Capabilities()
	require.True(t, caps.Multipart)

	objectKey := "objects/multipart/payload"
	body := bytes.Repeat([]byte("ab"), 5*1024*1024) // 10 MiB
	size := int64(len(body))

	upload, err := mp.CreateMultipart(ctx, objectKey, size)
	require.NoError(t, err)
	require.NotEmpty(t, upload.UploadID)
	require.Equal(t, objectKey, upload.ObjectKey)

	partSize := int64(5 * 1024 * 1024)
	part1 := body[:partSize]
	part2 := body[partSize:]

	url1, err := mp.PresignPart(ctx, upload.UploadID, 1, time.Minute)
	require.NoError(t, err)
	require.NotEmpty(t, url1)
	etag1, err := putPresignedPart(ctx, url1, part1)
	require.NoError(t, err)
	require.NotEmpty(t, etag1)

	listed, err := mp.ListParts(ctx, upload.UploadID)
	require.NoError(t, err)
	require.Len(t, listed, 1)
	require.Equal(t, 1, listed[0].PartNumber)
	require.Equal(t, normalizeETag(etag1), normalizeETag(listed[0].ETag))
	require.Equal(t, int64(len(part1)), listed[0].Size)

	url2, err := mp.PresignPart(ctx, upload.UploadID, 2, time.Minute)
	require.NoError(t, err)
	etag2, err := putPresignedPart(ctx, url2, part2)
	require.NoError(t, err)

	listed, err = mp.ListParts(ctx, upload.UploadID)
	require.NoError(t, err)
	require.Len(t, listed, 2)

	info, err := mp.CompleteMultipart(ctx, upload.UploadID, []storage.CompletedPart{
		{PartNumber: 1, ETag: etag1},
		{PartNumber: 2, ETag: etag2},
	})
	require.NoError(t, err)
	require.Equal(t, objectKey, info.ObjectKey)
	require.Equal(t, size, info.Size)
	if info.SHA256 != "" {
		require.Equal(t, storage.SHA256(body), info.SHA256)
	}

	again, err := mp.CompleteMultipart(ctx, upload.UploadID, []storage.CompletedPart{
		{PartNumber: 1, ETag: etag1},
		{PartNumber: 2, ETag: etag2},
	})
	require.NoError(t, err)
	require.Equal(t, info, again)

	uploadBad, err := mp.CreateMultipart(ctx, "objects/multipart/bad-etag", size)
	require.NoError(t, err)
	_, err = putPresignedPart(ctx, mustPresign(t, mp, ctx, uploadBad.UploadID, 1), part1)
	require.NoError(t, err)
	_, err = mp.CompleteMultipart(ctx, uploadBad.UploadID, []storage.CompletedPart{
		{PartNumber: 1, ETag: `"deadbeef"`},
	})
	require.ErrorIs(t, err, storage.ErrInvalidPart)
	require.NoError(t, mp.AbortMultipart(ctx, uploadBad.UploadID))

	abortUpload, err := mp.CreateMultipart(ctx, "objects/multipart/aborted", 1024)
	require.NoError(t, err)
	require.NoError(t, mp.AbortMultipart(ctx, abortUpload.UploadID))
	_, err = mp.ListParts(ctx, abortUpload.UploadID)
	require.Error(t, err)
}

func mustPresign(t *testing.T, mp storage.MultipartProvider, ctx context.Context, uploadID string, part int) string {
	t.Helper()
	url, err := mp.PresignPart(ctx, uploadID, part, time.Minute)
	require.NoError(t, err)
	return url
}

func putPresignedPart(ctx context.Context, url string, body []byte) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, url, bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		slurp, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return "", fmt.Errorf("presigned put failed: %s (%q)", resp.Status, slurp)
	}
	return normalizeETag(resp.Header.Get("ETag")), nil
}

func normalizeETag(value string) string {
	return strings.Trim(value, "\"")
}
