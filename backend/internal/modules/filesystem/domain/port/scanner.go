package port

import (
	"context"
	"errors"

	"github.com/google/uuid"
)

var (
	ErrScannerUnavailable = errors.New("filesystem scanner unavailable")
	ErrScannerInfected    = errors.New("filesystem scanner infected")
)

type ObjectRef struct {
	ProviderID uuid.UUID
	ObjectKey  string
}

type ScanResult struct {
	Clean       bool
	ThreatName  string
	Engine      string
	Signature   string
}

type Scanner interface {
	Scan(ctx context.Context, object ObjectRef) (ScanResult, error)
}

type NoopScanner struct {
	Result ScanResult
	Err    error
}

func (n NoopScanner) Scan(context.Context, ObjectRef) (ScanResult, error) {
	if n.Err != nil {
		return ScanResult{}, n.Err
	}
	result := n.Result
	if result.Engine == "" {
		result.Engine = "noop"
	}
	if n.Result == (ScanResult{}) {
		result.Clean = true
	}
	return result, nil
}

type RecordingScanner struct {
	Calls []ObjectRef
	Inner Scanner
}

func (r *RecordingScanner) Scan(ctx context.Context, object ObjectRef) (ScanResult, error) {
	r.Calls = append(r.Calls, object)
	if r.Inner == nil {
		return NoopScanner{}.Scan(ctx, object)
	}
	return r.Inner.Scan(ctx, object)
}
