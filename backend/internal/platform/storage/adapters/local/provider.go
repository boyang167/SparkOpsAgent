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
	"runtime"
	"strings"
	"sync"
	"unicode/utf8"

	"github.com/sparkops/sparkops-agent/backend/internal/platform/storage"
)

type Provider struct {
	root string
	mu   sync.Mutex
}

func New(root string) (*Provider, error) {
	if root == "" {
		return nil, fmt.Errorf("%w: empty root", storage.ErrInvalidObjectKey)
	}
	absolute, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(absolute, 0o750); err != nil {
		return nil, err
	}
	absolute, err = filepath.EvalSymlinks(absolute)
	if err != nil {
		return nil, err
	}
	return &Provider{root: filepath.Clean(absolute)}, nil
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
	path, err := p.resolve(req.ObjectKey, true)
	if err != nil {
		return storage.ObjectInfo{}, err
	}
	p.mu.Lock()
	defer p.mu.Unlock()

	existing, existingErr := p.statPath(req.ObjectKey, path)
	if existingErr != nil && !errors.Is(existingErr, storage.ErrNotFound) {
		return storage.ObjectInfo{}, existingErr
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return storage.ObjectInfo{}, err
	}
	if err := p.rejectSymlinkComponents(path, true); err != nil {
		return storage.ObjectInfo{}, err
	}
	temp, err := os.CreateTemp(filepath.Dir(path), ".storage-*")
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
	if err := os.Rename(tempPath, path); err != nil {
		if os.IsExist(err) {
			return storage.ObjectInfo{}, storage.ErrObjectConflict
		}
		return storage.ObjectInfo{}, err
	}
	published = true
	_ = syncDirectory(filepath.Dir(path))
	return storage.ObjectInfo{ObjectKey: req.ObjectKey, Size: written, SHA256: checksum}, nil
}

func (p *Provider) Get(ctx context.Context, req storage.GetRequest) (io.ReadCloser, storage.ObjectInfo, error) {
	if err := ctx.Err(); err != nil {
		return nil, storage.ObjectInfo{}, err
	}
	if req.Offset < 0 || req.Length < 0 {
		return nil, storage.ObjectInfo{}, storage.ErrInvalidRange
	}
	path, err := p.resolve(req.ObjectKey, false)
	if err != nil {
		return nil, storage.ObjectInfo{}, err
	}
	info, err := p.statPath(req.ObjectKey, path)
	if err != nil {
		return nil, storage.ObjectInfo{}, err
	}
	if req.Offset > info.Size || (req.Length > 0 && req.Offset+req.Length > info.Size) {
		return nil, storage.ObjectInfo{}, storage.ErrInvalidRange
	}
	file, err := os.Open(path)
	if err != nil {
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
	return &contextReadCloser{ctx: ctx, reader: reader, closer: file}, info, nil
}

func (p *Provider) Stat(_ context.Context, objectKey string) (storage.ObjectInfo, error) {
	path, err := p.resolve(objectKey, false)
	if err != nil {
		return storage.ObjectInfo{}, err
	}
	return p.statPath(objectKey, path)
}

func (p *Provider) Delete(_ context.Context, objectKey string) error {
	path, err := p.resolve(objectKey, false)
	if err != nil {
		return err
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if err := os.Remove(path); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return storage.ErrNotFound
		}
		return err
	}
	_ = syncDirectory(filepath.Dir(path))
	return nil
}

func (p *Provider) Copy(ctx context.Context, sourceKey, destinationKey string) (storage.ObjectInfo, error) {
	reader, info, err := p.Get(ctx, storage.GetRequest{ObjectKey: sourceKey})
	if err != nil {
		return storage.ObjectInfo{}, err
	}
	defer reader.Close()
	return p.Put(ctx, storage.PutRequest{
		ObjectKey: destinationKey, Body: reader, Size: info.Size, SHA256: info.SHA256,
	})
}

func (p *Provider) resolve(objectKey string, forWrite bool) (string, error) {
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
	clean := filepath.Clean(filepath.FromSlash(objectKey))
	if clean == "." || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return "", storage.ErrInvalidObjectKey
	}
	path := filepath.Join(p.root, clean)
	if !withinRoot(p.root, path) {
		return "", storage.ErrInvalidObjectKey
	}
	if err := p.rejectSymlinkComponents(path, forWrite); err != nil {
		return "", err
	}
	return path, nil
}

func (p *Provider) rejectSymlinkComponents(path string, allowMissingFinal bool) error {
	relative, err := filepath.Rel(p.root, path)
	if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return storage.ErrInvalidObjectKey
	}
	current := p.root
	parts := strings.Split(relative, string(filepath.Separator))
	for _, part := range parts {
		current = filepath.Join(current, part)
		info, statErr := os.Lstat(current)
		if errors.Is(statErr, os.ErrNotExist) && allowMissingFinal {
			break
		}
		if statErr != nil {
			if errors.Is(statErr, os.ErrNotExist) {
				return storage.ErrNotFound
			}
			return statErr
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return storage.ErrInvalidObjectKey
		}
	}
	return nil
}

func (p *Provider) statPath(objectKey, path string) (storage.ObjectInfo, error) {
	fileInfo, err := os.Stat(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return storage.ObjectInfo{}, storage.ErrNotFound
		}
		return storage.ObjectInfo{}, err
	}
	if !fileInfo.Mode().IsRegular() {
		return storage.ObjectInfo{}, storage.ErrNotFound
	}
	file, err := os.Open(path)
	if err != nil {
		return storage.ObjectInfo{}, err
	}
	defer file.Close()
	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return storage.ObjectInfo{}, err
	}
	return storage.ObjectInfo{ObjectKey: objectKey, Size: fileInfo.Size(), SHA256: hex.EncodeToString(hash.Sum(nil))}, nil
}

func withinRoot(root, path string) bool {
	relative, err := filepath.Rel(root, path)
	return err == nil && relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator))
}

func syncDirectory(path string) error {
	if runtime.GOOS == "windows" {
		return nil
	}
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()
	return file.Sync()
}

func copyWithContext(ctx context.Context, dst io.Writer, src io.Reader) (int64, error) {
	return io.Copy(dst, &contextReader{ctx: ctx, reader: src})
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
