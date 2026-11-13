package port

import (
	"context"
	"errors"
)

var (
	ErrPreviewUnavailable = errors.New("filesystem preview unavailable")
	ErrPreviewUnsupported = errors.New("filesystem preview unsupported")
)

type PreviewSource struct {
	MIMEType string
	Size     int64
	Bytes    []byte
	Object   ObjectRef
}

type PreviewResult struct {
	Bytes    []byte
	MIMEType string
	SHA256   string
	Size     int64
}

type PreviewRequest struct {
	Source         PreviewSource
	Classification string
}

type Previewer interface {
	Render(ctx context.Context, request PreviewRequest) (PreviewResult, error)
}

type NoopPreviewer struct {
	Result PreviewResult
	Err    error
}

func (n NoopPreviewer) Render(context.Context, PreviewRequest) (PreviewResult, error) {
	if n.Err != nil {
		return PreviewResult{}, n.Err
	}
	result := n.Result
	if len(result.Bytes) == 0 && result.MIMEType == "" {
		return PreviewResult{}, ErrPreviewUnsupported
	}
	if result.MIMEType == "" {
		result.MIMEType = "application/octet-stream"
	}
	if result.Size == 0 {
		result.Size = int64(len(result.Bytes))
	}
	return result, nil
}
