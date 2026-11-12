package local

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"unicode/utf8"

	"github.com/sparkops/sparkops-agent/backend/internal/platform/storage"
)

type Provider struct {
	root    string
	syncDir func(string) error
}

func New(root string) (*Provider, error) {
	if root == "" {
		return nil, fmt.Errorf("%w: empty root", storage.ErrInvalidObjectKey)
	}
	absolute, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	if info, statErr := os.Lstat(absolute); statErr == nil {
		if info.Mode()&os.ModeSymlink != 0 || isReparsePoint(absolute) {
			return nil, storage.ErrInvalidRoot
		}
	} else if !errors.Is(statErr, os.ErrNotExist) {
		return nil, statErr
	}
	if err := os.MkdirAll(absolute, 0o750); err != nil {
		return nil, err
	}
	absolute, err = filepath.EvalSymlinks(absolute)
	if err != nil {
		return nil, err
	}
	if isReparsePoint(absolute) {
		return nil, storage.ErrInvalidRoot
	}
	return &Provider{root: filepath.Clean(absolute), syncDir: syncDirectory}, nil
}

func (p *Provider) ID() string { return "local" }

func (p *Provider) Capabilities() storage.Capabilities {
	return storage.Capabilities{RangeRead: true, AtomicPublish: true, VerifiedCopy: true}
}

func (p *Provider) Put(ctx context.Context, req storage.PutRequest) (storage.ObjectInfo, error) {
	if err := ctx.Err(); err != nil {
		return storage.ObjectInfo{}, err
	}
	if req.Body == nil || req.Size < 0 {
		return storage.ObjectInfo{}, storage.ErrSizeMismatch
	}
	if err := validateChecksum(req.SHA256); err != nil {
		return storage.ObjectInfo{}, err
	}
	path, err := p.resolve(req.ObjectKey)
	if err != nil {
		return storage.ObjectInfo{}, err
	}

	existing, existingErr := p.statPath(ctx, req.ObjectKey, path)
	if existingErr != nil && !errors.Is(existingErr, storage.ErrNotFound) {
		return storage.ObjectInfo{}, existingErr
	}

	temp, err := os.CreateTemp(p.root, ".storage-*")
	if err != nil {
		return storage.ObjectInfo{}, err
	}
	tempPath := temp.Name()
	published := false
	defer func() {
		_ = temp.Close()
		if !published {
			_ = os.Remove(tempPath)
		}
	}()

	hash := sha256.New()
	written, err := copyWithContext(ctx, io.MultiWriter(temp, hash), req.Body)
	if err != nil {
		return storage.ObjectInfo{}, err
	}
	if written != req.Size {
		return storage.ObjectInfo{}, storage.ErrSizeMismatch
	}
	checksum := hex.EncodeToString(hash.Sum(nil))
	if req.SHA256 != "" && !strings.EqualFold(req.SHA256, checksum) {
		return storage.ObjectInfo{}, storage.ErrChecksumMismatch
	}
	if existingErr == nil {
		if existing.Size == written && existing.SHA256 == checksum {
			return existing, nil
		}
		return storage.ObjectInfo{}, storage.ErrObjectConflict
	}
	if err := temp.Sync(); err != nil {
		return storage.ObjectInfo{}, err
	}
	if err := temp.Close(); err != nil {
		return storage.ObjectInfo{}, err
	}
	if err := os.Link(tempPath, path); err != nil {
		if os.IsExist(err) {
			_ = os.Remove(tempPath)
			winner, statErr := p.statPath(ctx, req.ObjectKey, path)
			if statErr != nil {
				return storage.ObjectInfo{}, statErr
			}
			if winner.Size == written && winner.SHA256 == checksum {
				return winner, nil
			}
			return storage.ObjectInfo{}, storage.ErrObjectConflict
		}
		return storage.ObjectInfo{}, err
	}
	if err := os.Remove(tempPath); err != nil {
		return storage.ObjectInfo{}, err
	}
	published = true
	if err := p.syncDir(p.root); err != nil {
		return storage.ObjectInfo{}, err
	}
	return storage.ObjectInfo{ObjectKey: req.ObjectKey, Size: written, SHA256: checksum}, nil
}

func (p *Provider) Get(ctx context.Context, req storage.GetRequest) (io.ReadCloser, storage.ObjectInfo, error) {
	if err := ctx.Err(); err != nil {
		return nil, storage.ObjectInfo{}, err
	}
	if req.Offset < 0 || req.Length < 0 {
		return nil, storage.ObjectInfo{}, storage.ErrInvalidRange
	}
	path, err := p.resolve(req.ObjectKey)
	if err != nil {
		return nil, storage.ObjectInfo{}, err
	}
	info, err := p.statPath(ctx, req.ObjectKey, path)
	if err != nil {
		return nil, storage.ObjectInfo{}, err
	}
	if req.Offset > info.Size || (req.Length > info.Size-req.Offset) {
		return nil, storage.ObjectInfo{}, storage.ErrInvalidRange
	}
	file, err := openObject(path)
	if err != nil {
		if errors.Is(err, errUnsafePath) {
			return nil, storage.ObjectInfo{}, storage.ErrInvalidObjectKey
		}
		if errors.Is(err, os.ErrNotExist) {
			return nil, storage.ObjectInfo{}, storage.ErrNotFound
		}
		return nil, storage.ObjectInfo{}, err
	}
	if _, err := file.Seek(req.Offset, io.SeekStart); err != nil {
		_ = file.Close()
		return nil, storage.ObjectInfo{}, err
	}
	var reader io.Reader = file
	if req.Length > 0 {
		reader = io.LimitReader(reader, req.Length)
	}
	return newContextReadCloser(ctx, reader, file), info, nil
}

func (p *Provider) Stat(ctx context.Context, objectKey string) (storage.ObjectInfo, error) {
	if err := ctx.Err(); err != nil {
		return storage.ObjectInfo{}, err
	}
	path, err := p.resolve(objectKey)
	if err != nil {
		return storage.ObjectInfo{}, err
	}
	return p.statPath(ctx, objectKey, path)
}

func (p *Provider) Delete(ctx context.Context, objectKey string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	path, err := p.resolve(objectKey)
	if err != nil {
		return err
	}
	if err := os.Remove(path); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}
	if err := p.syncDir(p.root); err != nil {
		return err
	}
	return nil
}

func (p *Provider) Copy(ctx context.Context, sourceKey, destinationKey string) (storage.ObjectInfo, error) {
	if err := ctx.Err(); err != nil {
		return storage.ObjectInfo{}, err
	}
	reader, info, err := p.Get(ctx, storage.GetRequest{ObjectKey: sourceKey})
	if err != nil {
		return storage.ObjectInfo{}, err
	}
	defer reader.Close()
	return p.Put(ctx, storage.PutRequest{
		ObjectKey: destinationKey, Body: reader, Size: info.Size, SHA256: info.SHA256,
	})
}

func (p *Provider) resolve(objectKey string) (string, error) {
	if objectKey == "" || !utf8.ValidString(objectKey) || strings.IndexByte(objectKey, 0) >= 0 || filepath.IsAbs(objectKey) ||
		strings.HasPrefix(objectKey, "/") || strings.HasPrefix(objectKey, `\`) ||
		strings.Contains(objectKey, `\`) {
		return "", storage.ErrInvalidObjectKey
	}
	for _, component := range strings.Split(objectKey, "/") {
		if component == "." || component == ".." {
			return "", storage.ErrInvalidObjectKey
		}
	}
	return p.objectPath(objectKey), nil
}

func (p *Provider) objectPath(objectKey string) string {
	sum := sha256.Sum256([]byte(objectKey))
	return filepath.Join(p.root, hex.EncodeToString(sum[:])+".object")
}

func (p *Provider) statPath(ctx context.Context, objectKey, path string) (storage.ObjectInfo, error) {
	if err := ctx.Err(); err != nil {
		return storage.ObjectInfo{}, err
	}
	file, err := openObject(path)
	if err != nil {
		if errors.Is(err, errUnsafePath) {
			return storage.ObjectInfo{}, storage.ErrInvalidObjectKey
		}
		if errors.Is(err, os.ErrNotExist) {
			return storage.ObjectInfo{}, storage.ErrNotFound
		}
		return storage.ObjectInfo{}, err
	}
	fileInfo, err := file.Stat()
	if err != nil {
		_ = file.Close()
		return storage.ObjectInfo{}, err
	}
	if !fileInfo.Mode().IsRegular() {
		_ = file.Close()
		return storage.ObjectInfo{}, storage.ErrNotFound
	}
	defer file.Close()
	hash := sha256.New()
	if _, err := io.Copy(hash, &contextReader{ctx: ctx, reader: file}); err != nil {
		return storage.ObjectInfo{}, err
	}
	return storage.ObjectInfo{ObjectKey: objectKey, Size: fileInfo.Size(), SHA256: hex.EncodeToString(hash.Sum(nil))}, nil
}

func copyWithContext(ctx context.Context, dst io.Writer, src io.Reader) (int64, error) {
	return io.Copy(dst, &contextReader{ctx: ctx, reader: src})
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
	done   chan struct{}
	once   sync.Once
}

func newContextReadCloser(ctx context.Context, reader io.Reader, closer io.Closer) *contextReadCloser {
	result := &contextReadCloser{ctx: ctx, reader: reader, closer: closer, done: make(chan struct{})}
	go func() {
		select {
		case <-ctx.Done():
			_ = result.Close()
		case <-result.done:
		}
	}()
	return result
}

func (r *contextReadCloser) Read(p []byte) (int, error) {
	if err := r.ctx.Err(); err != nil {
		return 0, err
	}
	return r.reader.Read(p)
}

func (r *contextReadCloser) Close() error {
	var err error
	r.once.Do(func() {
		close(r.done)
		err = r.closer.Close()
	})
	return err
}
