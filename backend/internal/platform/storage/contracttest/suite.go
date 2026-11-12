package contracttest

import (
	"bytes"
	"context"
	"io"
	"math"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/sparkops/sparkops-agent/backend/internal/platform/storage"
)

func Run(t *testing.T, provider storage.Provider) {
	t.Helper()
	ctx := context.Background()
	caps := provider.Capabilities()
	require.True(t, caps.RangeRead)
	require.True(t, caps.AtomicPublish)
	require.True(t, caps.VerifiedCopy)
	key := "objects/contract/data"
	body := []byte("hello storage")
	sum := storage.SHA256(body)

	info, err := provider.Put(ctx, storage.PutRequest{
		ObjectKey: key, Body: bytes.NewReader(body), Size: int64(len(body)), SHA256: sum,
	})
	require.NoError(t, err)
	require.Equal(t, int64(len(body)), info.Size)
	require.Equal(t, sum, info.SHA256)
	upperInfo, err := provider.Put(ctx, storage.PutRequest{
		ObjectKey: "objects/contract/uppercase", Body: bytes.NewReader(body), Size: int64(len(body)), SHA256: strings.ToUpper(sum),
	})
	require.NoError(t, err)
	require.Equal(t, sum, upperInfo.SHA256)

	reader, got, err := provider.Get(ctx, storage.GetRequest{ObjectKey: key})
	require.NoError(t, err)
	gotBody, readErr := io.ReadAll(reader)
	require.NoError(t, reader.Close())
	require.NoError(t, readErr)
	require.Equal(t, body, gotBody)
	require.Equal(t, info, got)

	rangeReader, rangeInfo, err := provider.Get(ctx, storage.GetRequest{ObjectKey: key, Offset: 6, Length: 7})
	require.NoError(t, err)
	rangeBody, readErr := io.ReadAll(rangeReader)
	require.NoError(t, rangeReader.Close())
	require.NoError(t, readErr)
	require.Equal(t, []byte("storage"), rangeBody)
	require.Equal(t, info, rangeInfo)

	stat, err := provider.Stat(ctx, key)
	require.NoError(t, err)
	require.Equal(t, info, stat)

	again, err := provider.Put(ctx, storage.PutRequest{
		ObjectKey: key, Body: bytes.NewReader(body), Size: int64(len(body)), SHA256: sum,
	})
	require.NoError(t, err)
	require.Equal(t, info, again)

	var wg sync.WaitGroup
	errs := make(chan error, 4)
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, putErr := provider.Put(ctx, storage.PutRequest{
				ObjectKey: "objects/contract/concurrent", Body: bytes.NewReader(body), Size: int64(len(body)), SHA256: sum,
			})
			errs <- putErr
		}()
	}
	wg.Wait()
	close(errs)
	for putErr := range errs {
		require.NoError(t, putErr)
	}

	_, err = provider.Put(ctx, storage.PutRequest{
		ObjectKey: key, Body: bytes.NewReader([]byte("different")), Size: 9, SHA256: storage.SHA256([]byte("different")),
	})
	require.ErrorIs(t, err, storage.ErrObjectConflict)
	_, err = provider.Put(ctx, storage.PutRequest{
		ObjectKey: key, Body: bytes.NewReader([]byte("different dat")), Size: int64(len(body)),
	})
	require.ErrorIs(t, err, storage.ErrObjectConflict)

	_, err = provider.Put(ctx, storage.PutRequest{
		ObjectKey: "objects/contract/bad-size", Body: bytes.NewReader(body), Size: int64(len(body) + 1),
	})
	require.ErrorIs(t, err, storage.ErrSizeMismatch)

	_, err = provider.Put(ctx, storage.PutRequest{
		ObjectKey: "objects/contract/bad-checksum", Body: bytes.NewReader(body), Size: int64(len(body)),
		SHA256: strings.Repeat("0", 64),
	})
	require.ErrorIs(t, err, storage.ErrChecksumMismatch)

	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	_, err = provider.Put(cancelled, storage.PutRequest{
		ObjectKey: "objects/contract/cancelled", Body: bytes.NewReader(body), Size: int64(len(body)),
	})
	require.ErrorIs(t, err, context.Canceled)

	_, _, err = provider.Get(ctx, storage.GetRequest{ObjectKey: "objects/contract/missing"})
	require.ErrorIs(t, err, storage.ErrNotFound)

	empty, err := provider.Put(ctx, storage.PutRequest{
		ObjectKey: "objects/contract/empty", Body: bytes.NewReader(nil), Size: 0,
	})
	require.NoError(t, err)
	require.Equal(t, int64(0), empty.Size)

	emptyReader, _, err := provider.Get(ctx, storage.GetRequest{ObjectKey: "objects/contract/empty", Offset: 0, Length: 0})
	require.NoError(t, err)
	emptyBody, err := io.ReadAll(emptyReader)
	require.NoError(t, emptyReader.Close())
	require.NoError(t, err)
	require.Empty(t, emptyBody)

	_, _, err = provider.Get(ctx, storage.GetRequest{ObjectKey: key, Offset: -1, Length: 1})
	require.ErrorIs(t, err, storage.ErrInvalidRange)
	_, _, err = provider.Get(ctx, storage.GetRequest{ObjectKey: key, Offset: int64(len(body)), Length: 1})
	require.ErrorIs(t, err, storage.ErrInvalidRange)
	_, _, err = provider.Get(ctx, storage.GetRequest{ObjectKey: key, Offset: 1, Length: int64(len(body))})
	require.ErrorIs(t, err, storage.ErrInvalidRange)
	_, _, err = provider.Get(ctx, storage.GetRequest{ObjectKey: key, Offset: math.MaxInt64, Length: 2})
	require.ErrorIs(t, err, storage.ErrInvalidRange)
	_, _, err = provider.Get(ctx, storage.GetRequest{ObjectKey: key, Offset: 1, Length: math.MaxInt64})
	require.ErrorIs(t, err, storage.ErrInvalidRange)
	atEOF, _, err := provider.Get(ctx, storage.GetRequest{ObjectKey: key, Offset: int64(len(body)), Length: 0})
	require.NoError(t, err)
	eofBody, err := io.ReadAll(atEOF)
	require.NoError(t, atEOF.Close())
	require.NoError(t, err)
	require.Empty(t, eofBody)

	copyInfo, err := provider.Copy(ctx, key, "objects/contract/copy")
	require.NoError(t, err)
	require.Equal(t, storage.ObjectInfo{ObjectKey: "objects/contract/copy", Size: info.Size, SHA256: info.SHA256}, copyInfo)
	require.NoError(t, provider.Delete(ctx, "objects/contract/copy"))
	require.NoError(t, provider.Delete(ctx, "objects/contract/copy"))

	cancelGet, cancel := context.WithCancel(ctx)
	cancelReader, _, err := provider.Get(cancelGet, storage.GetRequest{ObjectKey: key})
	require.NoError(t, err)
	cancel()
	_, err = cancelReader.Read(make([]byte, 1))
	require.ErrorIs(t, err, context.Canceled)
	require.NoError(t, cancelReader.Close())
	require.NoError(t, cancelReader.Close())

	cancelled, cancel = context.WithCancel(ctx)
	cancel()
	_, err = provider.Stat(cancelled, key)
	require.ErrorIs(t, err, context.Canceled)
	require.ErrorIs(t, provider.Delete(cancelled, key), context.Canceled)
	_, err = provider.Copy(cancelled, key, "objects/contract/cancel-copy")
	require.ErrorIs(t, err, context.Canceled)
	_, err = provider.Put(ctx, storage.PutRequest{
		ObjectKey: "objects/contract/invalid-sha", Body: strings.NewReader("x"), Size: 1, SHA256: "abc",
	})
	require.ErrorIs(t, err, storage.ErrInvalidChecksum)

	for _, escaped := range []string{
		"../escape", "/absolute", `..\escape`, `objects\..\escape`, "objects/..",
		string([]byte{'o', 'b', 'j', 'e', 'c', 't', 's', '/', 0xff}),
	} {
		_, err := provider.Put(ctx, storage.PutRequest{
			ObjectKey: escaped, Body: bytes.NewReader(body), Size: int64(len(body)),
		})
		require.ErrorIs(t, err, storage.ErrInvalidObjectKey, escaped)
	}

	ctx, cancel = context.WithCancel(ctx)
	blocked := &cancelOnReadReader{cancel: cancel, data: bytes.Repeat([]byte("x"), 1024)}
	_, err = provider.Put(ctx, storage.PutRequest{
		ObjectKey: "objects/contract/stream-cancel", Body: blocked, Size: 1024,
	})
	require.ErrorIs(t, err, context.Canceled)
	_, err = provider.Stat(context.Background(), "objects/contract/stream-cancel")
	require.Error(t, err)

	require.NoError(t, provider.Delete(context.Background(), key))
	_, err = provider.Stat(context.Background(), key)
	require.ErrorIs(t, err, storage.ErrNotFound)
}

type cancelOnReadReader struct {
	cancel context.CancelFunc
	data   []byte
}

func (r *cancelOnReadReader) Read(p []byte) (int, error) {
	if len(r.data) == 0 {
		return 0, io.EOF
	}
	r.cancel()
	n := copy(p, r.data)
	r.data = r.data[n:]
	return n, nil
}
