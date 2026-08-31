package terminal

import (
	"errors"
	"io"
	"sync"

	"k8s.io/client-go/tools/remotecommand"
)

const maxCapturedWrites = 128

type Session struct {
	stdin    chan []byte
	sizes    chan remotecommand.TerminalSize
	done     chan struct{}
	closeOne sync.Once

	readMu  sync.Mutex
	pending []byte

	writeMu sync.Mutex
	writes  []ServerMessage
	onWrite func(ServerMessage)
}

func NewSession() *Session {
	return &Session{
		stdin: make(chan []byte, 64),
		sizes: make(chan remotecommand.TerminalSize, 8),
		done:  make(chan struct{}),
	}
}

func (s *Session) HandleClientJSON(raw []byte) error {
	message, err := ParseClientMessage(raw)
	if err != nil {
		return err
	}
	switch message.Type {
	case "read":
		select {
		case s.stdin <- []byte(message.Data):
			return nil
		case <-s.done:
			return io.EOF
		default:
			return errors.New("terminal input queue is full")
		}
	case "resize":
		size := remotecommand.TerminalSize{Width: message.Cols, Height: message.Rows}
		select {
		case s.sizes <- size:
			return nil
		case <-s.done:
			return io.EOF
		default:
		}
		select {
		case <-s.sizes:
		default:
		}
		select {
		case s.sizes <- size:
			return nil
		case <-s.done:
			return io.EOF
		default:
			return errors.New("terminal resize queue is full")
		}
	default:
		return io.ErrUnexpectedEOF
	}
}

func (s *Session) Read(p []byte) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}
	s.readMu.Lock()
	defer s.readMu.Unlock()

	if len(s.pending) == 0 {
		select {
		case next := <-s.stdin:
			s.pending = next
		case <-s.done:
			return 0, io.EOF
		}
	}
	n := copy(p, s.pending)
	s.pending = s.pending[n:]
	return n, nil
}

func (s *Session) Write(p []byte) (int, error) {
	select {
	case <-s.done:
		return 0, io.EOF
	default:
	}
	message := ServerMessage{Type: "write", Data: string(p)}
	s.writeMu.Lock()
	callback := s.onWrite
	if callback == nil && len(s.writes) < maxCapturedWrites {
		s.writes = append(s.writes, message)
	}
	s.writeMu.Unlock()
	if callback != nil {
		callback(message)
	}
	return len(p), nil
}

func (s *Session) Next() *remotecommand.TerminalSize {
	select {
	case size := <-s.sizes:
		return &size
	case <-s.done:
		return nil
	}
}

func (s *Session) SetOnWrite(callback func(ServerMessage)) {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	s.onWrite = callback
}

func (s *Session) TakeWrites() []ServerMessage {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	return append([]ServerMessage(nil), s.writes...)
}

func (s *Session) Close() {
	s.closeOne.Do(func() {
		close(s.done)
	})
}
