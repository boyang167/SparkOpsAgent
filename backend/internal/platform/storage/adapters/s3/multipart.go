package s3

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/storage"
)

func (p *Provider) CreateMultipart(ctx context.Context, objectKey string, size int64) (storage.MultipartUpload, error) {
	if err := ctx.Err(); err != nil {
		return storage.MultipartUpload{}, err
	}
	if err := validateObjectKey(objectKey); err != nil {
		return storage.MultipartUpload{}, err
	}
	if size < 0 {
		return storage.MultipartUpload{}, storage.ErrSizeMismatch
	}
	out, err := p.client.CreateMultipartUpload(ctx, &s3.CreateMultipartUploadInput{
		Bucket: aws.String(p.bucket),
		Key:    aws.String(objectKey),
		Metadata: map[string]string{
			metadataSHA256: "",
		},
	})
	if err != nil {
		return storage.MultipartUpload{}, err
	}
	uploadID := aws.ToString(out.UploadId)
	p.sessions.Store(uploadID, uploadSession{objectKey: objectKey, size: size})
	return storage.MultipartUpload{UploadID: uploadID, ObjectKey: objectKey}, nil
}

func (p *Provider) ListParts(ctx context.Context, uploadID string) ([]storage.UploadedPart, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	session, err := p.sessionFor(uploadID)
	if err != nil {
		return nil, err
	}
	out, err := p.client.ListParts(ctx, &s3.ListPartsInput{
		Bucket:   aws.String(p.bucket),
		Key:      aws.String(session.objectKey),
		UploadId: aws.String(uploadID),
	})
	if err != nil {
		return nil, mapMultipartNotFound(err)
	}
	parts := make([]storage.UploadedPart, 0, len(out.Parts))
	for _, part := range out.Parts {
		parts = append(parts, storage.UploadedPart{
			PartNumber: int(aws.ToInt32(part.PartNumber)),
			ETag:       strings.Trim(aws.ToString(part.ETag), "\""),
			Size:       aws.ToInt64(part.Size),
		})
	}
	return parts, nil
}

func (p *Provider) PresignPart(ctx context.Context, uploadID string, partNumber int, expires time.Duration) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	if partNumber < 1 {
		return "", storage.ErrInvalidPart
	}
	session, err := p.sessionFor(uploadID)
	if err != nil {
		return "", err
	}
	out, err := p.presign.PresignUploadPart(ctx, &s3.UploadPartInput{
		Bucket:     aws.String(p.bucket),
		Key:        aws.String(session.objectKey),
		UploadId:   aws.String(uploadID),
		PartNumber: aws.Int32(int32(partNumber)),
	}, s3.WithPresignExpires(expires))
	if err != nil {
		return "", err
	}
	return out.URL, nil
}

func (p *Provider) CompleteMultipart(ctx context.Context, uploadID string, parts []storage.CompletedPart) (storage.ObjectInfo, error) {
	if err := ctx.Err(); err != nil {
		return storage.ObjectInfo{}, err
	}
	session, err := p.sessionFor(uploadID)
	if err != nil {
		return storage.ObjectInfo{}, err
	}
	if session.completed != nil {
		return *session.completed, nil
	}
	completedParts := make([]types.CompletedPart, 0, len(parts))
	for _, part := range parts {
		if part.PartNumber < 1 || strings.TrimSpace(part.ETag) == "" {
			return storage.ObjectInfo{}, storage.ErrInvalidPart
		}
		completedParts = append(completedParts, types.CompletedPart{
			PartNumber: aws.Int32(int32(part.PartNumber)),
			ETag:       aws.String(quoteETag(part.ETag)),
		})
	}
	out, err := p.client.CompleteMultipartUpload(ctx, &s3.CompleteMultipartUploadInput{
		Bucket:   aws.String(p.bucket),
		Key:      aws.String(session.objectKey),
		UploadId: aws.String(uploadID),
		MultipartUpload: &types.CompletedMultipartUpload{
			Parts: completedParts,
		},
	})
	if err != nil {
		if isInvalidPart(err) {
			return storage.ObjectInfo{}, storage.ErrInvalidPart
		}
		if completed, statErr := p.statExisting(ctx, session.objectKey); statErr == nil {
			info := completed
			p.rememberCompleted(uploadID, info)
			return info, nil
		}
		return storage.ObjectInfo{}, mapMultipartNotFound(err)
	}
	size := session.size
	if size == 0 {
		if head, headErr := p.statExisting(ctx, session.objectKey); headErr == nil {
			size = head.Size
		}
	}
	info := storage.ObjectInfo{
		ObjectKey: session.objectKey,
		Size:      size,
		SHA256:    "",
	}
	if etag := aws.ToString(out.ETag); etag != "" {
		_ = etag
	}
	p.rememberCompleted(uploadID, info)
	return info, nil
}

func (p *Provider) AbortMultipart(ctx context.Context, uploadID string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	session, err := p.sessionFor(uploadID)
	if err != nil {
		return err
	}
	_, err = p.client.AbortMultipartUpload(ctx, &s3.AbortMultipartUploadInput{
		Bucket:   aws.String(p.bucket),
		Key:      aws.String(session.objectKey),
		UploadId: aws.String(uploadID),
	})
	if err != nil {
		return mapMultipartNotFound(err)
	}
	p.sessions.Delete(uploadID)
	return nil
}

func (p *Provider) sessionFor(uploadID string) (uploadSession, error) {
	if value, ok := p.sessions.Load(uploadID); ok {
		return value.(uploadSession), nil
	}
	return uploadSession{}, fmt.Errorf("%w: %s", storage.ErrMultipartNotFound, uploadID)
}

func (p *Provider) rememberCompleted(uploadID string, info storage.ObjectInfo) {
	if value, ok := p.sessions.Load(uploadID); ok {
		session := value.(uploadSession)
		session.completed = &info
		p.sessions.Store(uploadID, session)
	}
}

func quoteETag(value string) string {
	value = strings.Trim(value, "\"")
	if value == "" {
		return ""
	}
	return strconv.Quote(value)
}

func mapMultipartNotFound(err error) error {
	var noSuchUpload *types.NoSuchUpload
	if errors.As(err, &noSuchUpload) {
		return storage.ErrMultipartNotFound
	}
	return err
}

func isInvalidPart(err error) bool {
	var apiErr interface{ ErrorCode() string }
	if errors.As(err, &apiErr) {
		switch apiErr.ErrorCode() {
		case "InvalidPart", "MalformedXML", "InvalidRequest":
			return true
		}
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "invalid part") || strings.Contains(msg, "part number")
}
