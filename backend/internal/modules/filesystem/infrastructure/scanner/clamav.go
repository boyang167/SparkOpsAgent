package scanner

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"strings"
	"time"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/domain/port"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/storage"
)

type ClamAV struct {
	Addr       string
	Dial       func(context.Context, string, string) (net.Conn, error)
	OpenObject func(context.Context, port.ObjectRef) (io.ReadCloser, int64, error)
	MaxBytes   int64
	Timeout    time.Duration
}

func (c *ClamAV) Scan(ctx context.Context, object port.ObjectRef) (port.ScanResult, error) {
	if c == nil {
		return port.ScanResult{}, port.ErrScannerUnavailable
	}
	if c.OpenObject == nil || c.Dial == nil {
		return port.ScanResult{}, port.ErrScannerUnavailable
	}
	body, size, err := c.OpenObject(ctx, object)
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			return port.ScanResult{}, port.ErrScannerUnavailable
		}
		return port.ScanResult{}, err
	}
	defer body.Close()
	maxBytes := c.MaxBytes
	if maxBytes <= 0 {
		maxBytes = 64 * 1024 * 1024
	}
	if size > maxBytes {
		return port.ScanResult{}, port.ErrScannerUnavailable
	}
	timeout := c.Timeout
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	dialCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	conn, err := c.Dial(dialCtx, "tcp", c.Addr)
	if err != nil {
		return port.ScanResult{}, port.ErrScannerUnavailable
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(timeout))
	if _, err := conn.Write([]byte("zINSTREAM\000")); err != nil {
		return port.ScanResult{}, port.ErrScannerUnavailable
	}
	limited := io.LimitReader(body, maxBytes+1)
	buffer := make([]byte, 32*1024)
	for {
		n, readErr := limited.Read(buffer)
		if n > 0 {
			var header [4]byte
			binary.BigEndian.PutUint32(header[:], uint32(n))
			if _, err := conn.Write(header[:]); err != nil {
				return port.ScanResult{}, port.ErrScannerUnavailable
			}
			if _, err := conn.Write(buffer[:n]); err != nil {
				return port.ScanResult{}, port.ErrScannerUnavailable
			}
		}
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			return port.ScanResult{}, readErr
		}
	}
	if _, err := conn.Write([]byte{0, 0, 0, 0}); err != nil {
		return port.ScanResult{}, port.ErrScannerUnavailable
	}
	response := make([]byte, 512)
	n, err := conn.Read(response)
	if err != nil {
		return port.ScanResult{}, port.ErrScannerUnavailable
	}
	text := strings.TrimSpace(string(response[:n]))
	if strings.HasPrefix(text, "stream: OK") {
		return port.ScanResult{Clean: true, Engine: "clamav"}, nil
	}
	if strings.Contains(text, "FOUND") {
		threat := strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(text, "stream: "), "FOUND"))
		return port.ScanResult{Clean: false, ThreatName: threat, Engine: "clamav", Signature: threat}, port.ErrScannerInfected
	}
	return port.ScanResult{}, fmt.Errorf("%w: unexpected clamav response %q", port.ErrScannerUnavailable, text)
}
