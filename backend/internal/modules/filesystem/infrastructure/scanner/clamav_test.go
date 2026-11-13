package scanner

import (
	"bufio"
	"context"
	"encoding/binary"
	"io"
	"net"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/domain/port"
)

func TestClamAVScanMarksCleanResponses(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer listener.Close()
	go func() {
		conn, acceptErr := listener.Accept()
		if acceptErr != nil {
			return
		}
		defer conn.Close()
		reader := bufio.NewReader(conn)
		var command [10]byte
		if _, err := io.ReadFull(reader, command[:]); err != nil {
			return
		}
		for {
			var header [4]byte
			if _, err := io.ReadFull(reader, header[:]); err != nil {
				break
			}
			size := binary.BigEndian.Uint32(header[:])
			if size == 0 {
				break
			}
			if _, err := io.CopyN(io.Discard, reader, int64(size)); err != nil {
				break
			}
		}
		_, _ = conn.Write([]byte("stream: OK"))
	}()
	scanner := &ClamAV{
		Addr: listener.Addr().String(),
		Dial: func(ctx context.Context, network, address string) (net.Conn, error) {
			dialer := net.Dialer{}
			return dialer.DialContext(ctx, network, address)
		},
		OpenObject: func(context.Context, port.ObjectRef) (io.ReadCloser, int64, error) {
			return io.NopCloser(strings.NewReader("payload")), int64(len("payload")), nil
		},
	}
	result, err := scanner.Scan(context.Background(), port.ObjectRef{ProviderID: uuid.New(), ObjectKey: "objects/payload"})
	require.NoError(t, err)
	require.True(t, result.Clean)
	require.Equal(t, "clamav", result.Engine)
}

func TestClamAVScanMarksInfectedResponses(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer listener.Close()
	go func() {
		conn, acceptErr := listener.Accept()
		if acceptErr != nil {
			return
		}
		defer conn.Close()
		reader := bufio.NewReader(conn)
		var command [10]byte
		if _, err := io.ReadFull(reader, command[:]); err != nil {
			return
		}
		for {
			var header [4]byte
			if _, err := io.ReadFull(reader, header[:]); err != nil {
				break
			}
			size := binary.BigEndian.Uint32(header[:])
			if size == 0 {
				break
			}
			if _, err := io.CopyN(io.Discard, reader, int64(size)); err != nil {
				break
			}
		}
		_, _ = conn.Write([]byte("stream: Eicar-Test-Signature FOUND"))
	}()
	scanner := &ClamAV{
		Addr: listener.Addr().String(),
		Dial: func(ctx context.Context, network, address string) (net.Conn, error) {
			dialer := net.Dialer{}
			return dialer.DialContext(ctx, network, address)
		},
		OpenObject: func(context.Context, port.ObjectRef) (io.ReadCloser, int64, error) {
			return io.NopCloser(strings.NewReader("payload")), int64(len("payload")), nil
		},
	}
	_, err = scanner.Scan(context.Background(), port.ObjectRef{ProviderID: uuid.New(), ObjectKey: "objects/payload"})
	require.ErrorIs(t, err, port.ErrScannerInfected)
}
