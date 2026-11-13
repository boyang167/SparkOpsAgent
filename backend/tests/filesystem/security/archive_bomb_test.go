//go:build integration

package security_test

import (
	"bytes"
	"context"
	"io"
	"net"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/domain/port"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/infrastructure/scanner"
	testenv "github.com/sparkops/sparkops-agent/backend/tests/filesystem/integration"
)

func TestScannerRejectsOversizedArchiveBeforeStreaming(t *testing.T) {
	testenv.Require(t)
	dialed := false
	clam := &scanner.ClamAV{
		MaxBytes: 1024,
		OpenObject: func(context.Context, port.ObjectRef) (io.ReadCloser, int64, error) {
			return io.NopCloser(bytes.NewReader(make([]byte, 2048))), 2048, nil
		},
		Dial: func(context.Context, string, string) (net.Conn, error) {
			dialed = true
			return nil, nil
		},
	}

	_, err := clam.Scan(context.Background(), port.ObjectRef{
		ProviderID: uuid.New(), ObjectKey: "archives/bomb.zip",
	})

	require.ErrorIs(t, err, port.ErrScannerUnavailable)
	require.False(t, dialed, "oversized content must be rejected before opening a scanner stream")
}
