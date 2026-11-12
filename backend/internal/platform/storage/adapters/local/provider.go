package local

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
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
	root        string
	guard       rootGuard
	digest      func(string) string
	syncDir     func() error
	removeTemp  func(string) error
	checkRootFn func() error
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
	absolute = filepath.Clean(absolute)
	guard, err := openRoot(absolute)
	if err != nil {
		if errors.Is(err, errRootChanged) {
			return nil, storage.ErrInvalidRoot
		}
		return nil, err
	}
	provider := &Provider{
		root: absolute, guard: guard, digest: func(key string) string {
			sum := sha256.Sum256([]byte(key))
			return hex.EncodeToString(sum[:])
		},
		syncDir: guard.sync,
	}
	provider.removeTemp = func(path string) error {
		return provider.guard.remove(filepath.Base(path))
	}
	provider.checkRootFn = guard.verify
	return provider, nil
}

func (p *Provider) ID() string { return "local" }

func (p *Provider) Close() error { return p.guard.close() }

func (p *Provider) Capabilities() storage.Capabilities {
	return storage.Capabilities{RangeRead: true, AtomicPublish: true, VerifiedCopy: true}
}

func (p *Provider) Put(ctx context.Context, req storage.PutRequest) (storage.ObjectInfo, error) {
	if err := ctx.Err(); err != nil {
		return storage.ObjectInfo{}, err
	}
	if err := p.checkRoot(); err != nil {
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

	existing, _, existingErr := p.statPath(ctx, req.ObjectKey, path)
	if existingErr != nil && !errors.Is(existingErr, storage.ErrNotFound) {
		return storage.ObjectInfo{}, existingErr
	}

	temp, tempName, err := p.guard.temp()
	if err != nil {
		return storage.ObjectInfo{}, err
	}
	tempPath := temp.Name()
	published := false
	defer func() {
		_ = temp.Close()
		if !published {
			_ = p.removeTemp(tempPath)
		}
	}()

	_, err = writeHeader(temp, req.ObjectKey)
	if err != nil {
		return storage.ObjectInfo{}, err
	}
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
	if err := p.checkRoot(); err != nil {
		return storage.ObjectInfo{}, err
	}
	objectName := filepath.Base(path)
	if err := p.guard.link(tempName, objectName); err != nil {
		if os.IsExist(err) {
			_ = p.removeTemp(tempPath)
			winner, _, statErr := p.statPath(ctx, req.ObjectKey, path)
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
	published = true
	_ = p.removeTemp(tempPath)
	if err := p.checkRoot(); err != nil {
		return storage.ObjectInfo{}, fmt.Errorf("%w: %v", storage.ErrOutcomeUnknown, err)
	}
	if err := p.syncDir(); err != nil {
		return storage.ObjectInfo{}, fmt.Errorf("%w: %v", storage.ErrOutcomeUnknown, err)
	}
	return storage.ObjectInfo{ObjectKey: req.ObjectKey, Size: written, SHA256: checksum}, nil
}

func (p *Provider) Get(ctx context.Context, req storage.GetRequest) (io.ReadCloser, storage.ObjectInfo, error) {
	if err := ctx.Err(); err != nil {
		return nil, storage.ObjectInfo{}, err
	}
	if err := p.checkRoot(); err != nil {
		return nil, storage.ObjectInfo{}, err
	}
	if req.Offset < 0 || req.Length < 0 {
		return nil, storage.ObjectInfo{}, storage.ErrInvalidRange
	}
	path, err := p.resolve(req.ObjectKey)
	if err != nil {
		return nil, storage.ObjectInfo{}, err
	}
	info, payloadOffset, err := p.statPath(ctx, req.ObjectKey, path)
	if err != nil {
		return nil, storage.ObjectInfo{}, err
	}
	if req.Offset > info.Size || (req.Length > info.Size-req.Offset) {
		return nil, storage.ObjectInfo{}, storage.ErrInvalidRange
	}
	file, err := p.guard.open(filepath.Base(path))
	if err != nil {
		if errors.Is(err, errUnsafePath) {
			return nil, storage.ObjectInfo{}, storage.ErrInvalidObjectKey
		}
		if errors.Is(err, os.ErrNotExist) {
			return nil, storage.ObjectInfo{}, storage.ErrNotFound
		}
		return nil, storage.ObjectInfo{}, err
	}
	storedKey, reopenedOffset, err := readHeader(file)
	if err != nil {
		_ = file.Close()
		return nil, storage.ObjectInfo{}, err
	}
	if storedKey != req.ObjectKey {
		_ = file.Close()
		return nil, storage.ObjectInfo{}, storage.ErrObjectKeyCollision
	}
	if reopenedOffset != payloadOffset {
		_ = file.Close()
		return nil, storage.ObjectInfo{}, storage.ErrCorruptObject
	}
	if req.Offset > int64(^uint64(0)>>1)-payloadOffset {
		_ = file.Close()
		return nil, storage.ObjectInfo{}, storage.ErrInvalidRange
	}
	if _, err := file.Seek(payloadOffset+req.Offset, io.SeekStart); err != nil {
		_ = file.Close()
		return nil, storage.ObjectInfo{}, err
	}
	var reader io.Reader = file
	if req.Length > 0 {
		reader = io.LimitReader(reader, req.Length)
	}
	if err := p.checkRoot(); err != nil {
		_ = file.Close()
		return nil, storage.ObjectInfo{}, err
	}
	return newContextReadCloser(ctx, reader, file), info, nil
}

func (p *Provider) Stat(ctx context.Context, objectKey string) (storage.ObjectInfo, error) {
	if err := ctx.Err(); err != nil {
		return storage.ObjectInfo{}, err
	}
	if err := p.checkRoot(); err != nil {
		return storage.ObjectInfo{}, err
	}
	path, err := p.resolve(objectKey)
	if err != nil {
		return storage.ObjectInfo{}, err
	}
	info, _, err := p.statPath(ctx, objectKey, path)
	return info, err
}

func (p *Provider) Delete(ctx context.Context, objectKey string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := p.checkRoot(); err != nil {
		return err
	}
	path, err := p.resolve(objectKey)
	if err != nil {
		return err
	}
	if _, _, err := p.statPath(ctx, objectKey, path); err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			return nil
		}
		return err
	}
	if err := p.guard.remove(filepath.Base(path)); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}
	if err := p.checkRoot(); err != nil {
		return fmt.Errorf("%w: %v", storage.ErrOutcomeUnknown, err)
	}
	if err := p.syncDir(); err != nil {
		return fmt.Errorf("%w: %v", storage.ErrOutcomeUnknown, err)
	}
	return nil
}

func (p *Provider) Copy(ctx context.Context, sourceKey, destinationKey string) (storage.ObjectInfo, error) {
	if err := ctx.Err(); err != nil {
		return storage.ObjectInfo{}, err
	}
	if err := p.checkRoot(); err != nil {
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
	return filepath.Join(p.root, p.digest(objectKey)+".object")
}

func (p *Provider) checkRoot() error {
	check := p.checkRootFn
	if check == nil {
		check = p.guard.verify
	}
	if err := check(); err != nil {
		return storage.ErrInvalidRoot
	}
	return nil
}

const (
	headerMagic   = "SPARKOBJ\x01"
	maxObjectKey  = 1 << 20
	headerKeySize = 4
)

func writeHeader(dst io.Writer, objectKey string) (int64, error) {
	if len(objectKey) > maxObjectKey {
		return 0, storage.ErrInvalidObjectKey
	}
	if _, err := io.WriteString(dst, headerMagic); err != nil {
		return 0, err
	}
	var size [headerKeySize]byte
	binary.BigEndian.PutUint32(size[:], uint32(len(objectKey)))
	if _, err := dst.Write(size[:]); err != nil {
		return 0, err
	}
	if _, err := io.WriteString(dst, objectKey); err != nil {
		return 0, err
	}
	return int64(len(headerMagic) + headerKeySize + len(objectKey)), nil
}

func readHeader(src io.Reader) (string, int64, error) {
	magic := make([]byte, len(headerMagic))
	if _, err := io.ReadFull(src, magic); err != nil {
		return "", 0, storage.ErrCorruptObject
	}
	if string(magic) != headerMagic {
		return "", 0, storage.ErrCorruptObject
	}
	var size [headerKeySize]byte
	if _, err := io.ReadFull(src, size[:]); err != nil {
		return "", 0, storage.ErrCorruptObject
	}
	keySize := binary.BigEndian.Uint32(size[:])
	if keySize > maxObjectKey {
		return "", 0, storage.ErrCorruptObject
	}
	key := make([]byte, keySize)
	if _, err := io.ReadFull(src, key); err != nil {
		return "", 0, storage.ErrCorruptObject
	}
	return string(key), int64(len(headerMagic)+headerKeySize) + int64(keySize), nil
}

func (p *Provider) statPath(ctx context.Context, objectKey, path string) (storage.ObjectInfo, int64, error) {
	if err := ctx.Err(); err != nil {
		return storage.ObjectInfo{}, 0, err
	}
	if err := p.checkRoot(); err != nil {
		return storage.ObjectInfo{}, 0, err
	}
	file, err := p.guard.open(filepath.Base(path))
	if err != nil {
		if errors.Is(err, errUnsafePath) {
			return storage.ObjectInfo{}, 0, storage.ErrInvalidObjectKey
		}
		if errors.Is(err, os.ErrNotExist) {
			return storage.ObjectInfo{}, 0, storage.ErrNotFound
		}
		return storage.ObjectInfo{}, 0, err
	}
	fileInfo, err := file.Stat()
	if err != nil {
		_ = file.Close()
		return storage.ObjectInfo{}, 0, err
	}
	if !fileInfo.Mode().IsRegular() {
		_ = file.Close()
		return storage.ObjectInfo{}, 0, storage.ErrNotFound
	}
	defer file.Close()
	storedKey, payloadOffset, err := readHeader(file)
	if err != nil {
		return storage.ObjectInfo{}, 0, err
	}
	if storedKey != objectKey {
		return storage.ObjectInfo{}, 0, storage.ErrObjectKeyCollision
	}
	if fileInfo.Size() < payloadOffset {
		return storage.ObjectInfo{}, 0, storage.ErrCorruptObject
	}
	hash := sha256.New()
	if _, err := io.Copy(hash, &contextReader{ctx: ctx, reader: file}); err != nil {
		return storage.ObjectInfo{}, 0, err
	}
	if err := p.checkRoot(); err != nil {
		return storage.ObjectInfo{}, 0, err
	}
	return storage.ObjectInfo{ObjectKey: objectKey, Size: fileInfo.Size() - payloadOffset, SHA256: hex.EncodeToString(hash.Sum(nil))}, payloadOffset, nil
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
	stop   func() bool
	once   sync.Once
}

func newContextReadCloser(ctx context.Context, reader io.Reader, closer io.Closer) *contextReadCloser {
	result := &contextReadCloser{ctx: ctx, reader: reader, closer: closer}
	result.stop = context.AfterFunc(ctx, func() { _ = result.closeUnderlying() })
	return result
}

func (r *contextReadCloser) Read(p []byte) (int, error) {
	if err := r.ctx.Err(); err != nil {
		return 0, err
	}
	return r.reader.Read(p)
}

func (r *contextReadCloser) Close() error {
	if r.stop != nil {
		r.stop()
	}
	return r.closeUnderlying()
}

func (r *contextReadCloser) closeUnderlying() error {
	var err error
	r.once.Do(func() {
		err = r.closer.Close()
	})
	return err
}
