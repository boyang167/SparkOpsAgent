package terminal

import (
	"io"
	"testing"

	"github.com/stretchr/testify/require"
	"k8s.io/client-go/tools/remotecommand"
)

func TestSessionMapsReadResizeAndWrite(t *testing.T) {
	session := NewSession()
	require.NoError(t, session.HandleClientJSON([]byte(`{"type":"read","data":"pwd\r"}`)))
	buf := make([]byte, 16)
	n, err := session.Read(buf)
	require.NoError(t, err)
	require.Equal(t, "pwd\r", string(buf[:n]))

	require.NoError(t, session.HandleClientJSON([]byte(`{"type":"resize","cols":120,"rows":40}`)))
	require.Equal(t, &remotecommand.TerminalSize{Width: 120, Height: 40}, session.Next())

	n, err = session.Write([]byte("hello"))
	require.NoError(t, err)
	require.Equal(t, 5, n)
	require.Equal(t, []ServerMessage{{Type: "write", Data: "hello"}}, session.TakeWrites())
}

func TestSessionPreservesInputLargerThanReadBuffer(t *testing.T) {
	session := NewSession()
	require.NoError(t, session.HandleClientJSON([]byte(`{"type":"read","data":"abcdef"}`)))
	buf := make([]byte, 3)
	n, err := session.Read(buf)
	require.NoError(t, err)
	require.Equal(t, "abc", string(buf[:n]))
	n, err = session.Read(buf)
	require.NoError(t, err)
	require.Equal(t, "def", string(buf[:n]))
}

func TestSessionCloseUnblocksReadAndNext(t *testing.T) {
	session := NewSession()
	session.Close()
	_, err := session.Read(make([]byte, 8))
	require.ErrorIs(t, err, io.EOF)
	require.Nil(t, session.Next())
}

func TestSessionBoundsCapturedWrites(t *testing.T) {
	session := NewSession()
	for i := 0; i < maxCapturedWrites+20; i++ {
		_, err := session.Write([]byte("line"))
		require.NoError(t, err)
	}
	require.Len(t, session.TakeWrites(), maxCapturedWrites)
}

func TestSessionRejectsInputWhenConsumerIsBackedUp(t *testing.T) {
	session := NewSession()
	for i := 0; i < cap(session.stdin); i++ {
		require.NoError(t, session.HandleClientJSON([]byte(`{"type":"read","data":"x"}`)))
	}
	require.Error(t, session.HandleClientJSON([]byte(`{"type":"read","data":"overflow"}`)))
}

func TestSessionKeepsLatestResizeWhenQueueIsFull(t *testing.T) {
	session := NewSession()
	for i := 0; i < cap(session.sizes)+1; i++ {
		message := []byte(`{"type":"resize","cols":` + intString(80+i) + `,"rows":24}`)
		require.NoError(t, session.HandleClientJSON(message))
	}
	var last *remotecommand.TerminalSize
	for i := 0; i < cap(session.sizes); i++ {
		last = session.Next()
	}
	require.Equal(t, uint16(80+cap(session.sizes)), last.Width)
}
