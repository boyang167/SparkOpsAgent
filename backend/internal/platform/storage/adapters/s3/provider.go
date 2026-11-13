package s3

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/storage"
)

const metadataSHA256 = "sha256"

type Config struct {
	Endpoint  string
	Region    string
	Bucket    string
	PathStyle bool
	AccessKey string
	SecretKey string
}

type Provider struct {
	bucket   string
	client   *s3.Client
	presign  *s3.PresignClient
	sessions sync.Map // uploadID -> uploadSession
}

type uploadSession struct {
	objectKey string
	size      int64
	completed *storage.ObjectInfo
}

func New(cfg Config) (*Provider, error) {
	if strings.TrimSpace(cfg.Bucket) == "" {
		return nil, fmt.Errorf("%w: empty bucket", storage.ErrInvalidObjectKey)
	}
	if strings.TrimSpace(cfg.Region) == "" {
		cfg.Region = "us-east-1"
	}
	loadOpts := []func(*config.LoadOptions) error{}
	if cfg.AccessKey != "" && cfg.SecretKey != "" {
		loadOpts = append(loadOpts, config.WithCredentialsProvider(
			credentials.NewStaticCredentialsProvider(cfg.AccessKey, cfg.SecretKey, ""),
		))
	}
	awsCfg, err := config.LoadDefaultConfig(context.Background(), append(loadOpts,
		config.WithRegion(cfg.Region),
	)...)
	if err != nil {
		return nil, err
	}
	client := s3.NewFromConfig(awsCfg, func(o *s3.Options) {
		if cfg.Endpoint != "" {
			o.BaseEndpoint = aws.String(cfg.Endpoint)
		}
		o.UsePathStyle = cfg.PathStyle
	})
	return &Provider{
		bucket:  cfg.Bucket,
		client:  client,
		presign: s3.NewPresignClient(client),
	}, nil
}

func (p *Provider) ID() string { return "s3" }

func (p *Provider) Capabilities() storage.Capabilities {
	return storage.Capabilities{
		RangeRead: true, AtomicPublish: true, VerifiedCopy: true,
		Multipart: true, PresignedUpload: true, POSIX: false,
	}
}

func (p *Provider) Put(ctx context.Context, req storage.PutRequest) (storage.ObjectInfo, error) {
	if err := ctx.Err(); err != nil {
		return storage.ObjectInfo{}, err
	}
	if err := validateObjectKey(req.ObjectKey); err != nil {
		return storage.ObjectInfo{}, err
	}
	if req.Body == nil || req.Size < 0 {
		return storage.ObjectInfo{}, storage.ErrSizeMismatch
	}
	if err := validateChecksum(req.SHA256); err != nil {
		return storage.ObjectInfo{}, err
	}
	existing, existErr := p.statExisting(ctx, req.ObjectKey)
	if existErr != nil && !errors.Is(existErr, storage.ErrNotFound) {
		return storage.ObjectInfo{}, existErr
	}
	body, err := readAllWithContext(ctx, req.Body, req.Size)
	if err != nil {
		return storage.ObjectInfo{}, err
	}
	if int64(len(body)) != req.Size {
		return storage.ObjectInfo{}, storage.ErrSizeMismatch
	}
	checksum := storage.SHA256(body)
	if req.SHA256 != "" && !strings.EqualFold(req.SHA256, checksum) {
		return storage.ObjectInfo{}, storage.ErrChecksumMismatch
	}
	if existErr == nil {
		if existing.Size == req.Size && strings.EqualFold(existing.SHA256, checksum) {
			return existing, nil
		}
		return storage.ObjectInfo{}, storage.ErrObjectConflict
	}
	_, err = p.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:        aws.String(p.bucket),
		Key:           aws.String(req.ObjectKey),
		Body:          bytes.NewReader(body),
		ContentLength: aws.Int64(req.Size),
		Metadata:      map[string]string{metadataSHA256: checksum},
	})
	if err != nil {
		return storage.ObjectInfo{}, err
	}
	return storage.ObjectInfo{ObjectKey: req.ObjectKey, Size: req.Size, SHA256: checksum}, nil
}

func (p *Provider) Get(ctx context.Context, req storage.GetRequest) (io.ReadCloser, storage.ObjectInfo, error) {
	if err := ctx.Err(); err != nil {
		return nil, storage.ObjectInfo{}, err
	}
	if err := validateObjectKey(req.ObjectKey); err != nil {
		return nil, storage.ObjectInfo{}, err
	}
	if req.Offset < 0 || req.Length < 0 {
		return nil, storage.ObjectInfo{}, storage.ErrInvalidRange
	}
	info, err := p.Stat(ctx, req.ObjectKey)
	if err != nil {
		return nil, storage.ObjectInfo{}, err
	}
	if req.Offset > info.Size || (req.Length > 0 && req.Length > info.Size-req.Offset) {
		return nil, storage.ObjectInfo{}, storage.ErrInvalidRange
	}
	if req.Offset == info.Size && req.Length == 0 {
		return io.NopCloser(bytes.NewReader(nil)), info, nil
	}
	input := &s3.GetObjectInput{Bucket: aws.String(p.bucket), Key: aws.String(req.ObjectKey)}
	if req.Length > 0 || req.Offset > 0 {
		var end string
		if req.Length > 0 {
			end = fmt.Sprintf("%d", req.Offset+req.Length-1)
		} else {
			end = ""
		}
		rng := fmt.Sprintf("bytes=%d-", req.Offset)
		if end != "" {
			rng = fmt.Sprintf("bytes=%d-%s", req.Offset, end)
		}
		input.Range = aws.String(rng)
	}
	out, err := p.client.GetObject(ctx, input)
	if err != nil {
		return nil, storage.ObjectInfo{}, mapS3NotFound(err)
	}
	return &contextReadCloser{ctx: ctx, closer: out.Body, reader: &contextReader{ctx: ctx, reader: out.Body}}, info, nil
}

func (p *Provider) Stat(ctx context.Context, objectKey string) (storage.ObjectInfo, error) {
	if err := ctx.Err(); err != nil {
		return storage.ObjectInfo{}, err
	}
	if err := validateObjectKey(objectKey); err != nil {
		return storage.ObjectInfo{}, err
	}
	return p.statExisting(ctx, objectKey)
}

func (p *Provider) Delete(ctx context.Context, objectKey string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := validateObjectKey(objectKey); err != nil {
		return err
	}
	_, err := p.statExisting(ctx, objectKey)
	if errors.Is(err, storage.ErrNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	_, err = p.client.DeleteObject(ctx, &s3.DeleteObjectInput{Bucket: aws.String(p.bucket), Key: aws.String(objectKey)})
	return err
}

func (p *Provider) Copy(ctx context.Context, sourceKey, destinationKey string) (storage.ObjectInfo, error) {
	if err := ctx.Err(); err != nil {
		return storage.ObjectInfo{}, err
	}
	if err := validateObjectKey(sourceKey); err != nil {
		return storage.ObjectInfo{}, err
	}
	if err := validateObjectKey(destinationKey); err != nil {
		return storage.ObjectInfo{}, err
	}
	source, err := p.Stat(ctx, sourceKey)
	if err != nil {
		return storage.ObjectInfo{}, err
	}
	dest, destErr := p.statExisting(ctx, destinationKey)
	if destErr != nil && !errors.Is(destErr, storage.ErrNotFound) {
		return storage.ObjectInfo{}, destErr
	}
	if destErr == nil {
		if dest.Size == source.Size && strings.EqualFold(dest.SHA256, source.SHA256) {
			return dest, nil
		}
		return storage.ObjectInfo{}, storage.ErrObjectConflict
	}
	copySource := fmt.Sprintf("%s/%s", p.bucket, sourceKey)
	_, err = p.client.CopyObject(ctx, &s3.CopyObjectInput{
		Bucket:     aws.String(p.bucket),
		Key:        aws.String(destinationKey),
		CopySource: aws.String(copySource),
		MetadataDirective: types.MetadataDirectiveCopy,
	})
	if err != nil {
		return storage.ObjectInfo{}, err
	}
	return storage.ObjectInfo{ObjectKey: destinationKey, Size: source.Size, SHA256: source.SHA256}, nil
}

func (p *Provider) PresignGet(ctx context.Context, objectKey string, expires time.Duration) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	if err := validateObjectKey(objectKey); err != nil {
		return "", err
	}
	out, err := p.presign.PresignGetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(p.bucket),
		Key:    aws.String(objectKey),
	}, s3.WithPresignExpires(expires))
	if err != nil {
		return "", err
	}
	return out.URL, nil
}

func (p *Provider) statExisting(ctx context.Context, objectKey string) (storage.ObjectInfo, error) {
	out, err := p.client.HeadObject(ctx, &s3.HeadObjectInput{Bucket: aws.String(p.bucket), Key: aws.String(objectKey)})
	if err != nil {
		return storage.ObjectInfo{}, mapS3NotFound(err)
	}
	sha := ""
	if out.Metadata != nil {
		sha = out.Metadata[metadataSHA256]
	}
	return storage.ObjectInfo{
		ObjectKey: objectKey,
		Size:      aws.ToInt64(out.ContentLength),
		SHA256:    strings.ToLower(sha),
	}, nil
}

func validateObjectKey(objectKey string) error {
	if objectKey == "" || !utf8.ValidString(objectKey) || strings.IndexByte(objectKey, 0) >= 0 ||
		strings.Contains(objectKey, "\\") || strings.HasPrefix(objectKey, "/") {
		return storage.ErrInvalidObjectKey
	}
	for _, part := range strings.Split(objectKey, "/") {
		if part == "." || part == ".." || part == "" {
			return storage.ErrInvalidObjectKey
		}
	}
	return nil
}

func validateChecksum(value string) error {
	if value == "" {
		return nil
	}
	if len(value) != sha256.Size*2 {
		return storage.ErrInvalidChecksum
	}
	if _, err := hex.DecodeString(value); err != nil {
		return storage.ErrInvalidChecksum
	}
	return nil
}

func readAllWithContext(ctx context.Context, src io.Reader, size int64) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	written, err := io.Copy(&buf, &contextReader{ctx: ctx, reader: io.LimitReader(src, size+1)})
	if err != nil {
		return nil, err
	}
	if written != size {
		return nil, storage.ErrSizeMismatch
	}
	return buf.Bytes(), nil
}

type contextReader struct {
	ctx    context.Context
	reader io.Reader
}

func (r *contextReader) Read(p []byte) (int, error) {
	if err := r.ctx.Err(); err != nil {
		return 0, err
	}
	n, err := r.reader.Read(p)
	if ctxErr := r.ctx.Err(); ctxErr != nil {
		return n, ctxErr
	}
	return n, err
}

type contextReadCloser struct {
	ctx    context.Context
	reader io.Reader
	closer io.Closer
}

func (r *contextReadCloser) Read(p []byte) (int, error) {
	if err := r.ctx.Err(); err != nil {
		return 0, err
	}
	return r.reader.Read(p)
}

func (r *contextReadCloser) Close() error { return r.closer.Close() }

func mapS3NotFound(err error) error {
	var notFound *types.NotFound
	var noSuchKey *types.NoSuchKey
	if errors.As(err, &notFound) || errors.As(err, &noSuchKey) {
		return storage.ErrNotFound
	}
	return err
}
