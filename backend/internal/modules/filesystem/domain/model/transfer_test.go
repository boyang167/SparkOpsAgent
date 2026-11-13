package model

import (
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func validTransferInput() TransferSessionInput {
	return TransferSessionInput{
		ID:                 uuid.New(),
		WorkspaceID:        uuid.New(),
		NamespaceID:        uuid.New(),
		CreatedBy:          uuid.New(),
		TargetName:         "report.pdf",
		Protocol:           TransferProtocolMultipart,
		ExpectedSize:       1024,
		ExpectedSHA256:     "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
		IdempotencyKey:     "upload-1",
		TemporaryObjectKey: "tmp/upload-1",
		ExpiresAt:          time.Now().Add(time.Hour),
		Classification:     TransferClassificationRestricted,
	}
}

func TestTransferStateMachine(t *testing.T) {
	session, err := NewTransferSession(validTransferInput())
	require.NoError(t, err)

	require.NoError(t, session.StartUpload())
	require.NoError(t, session.Pause())
	require.NoError(t, session.Resume())
	require.NoError(t, session.StartVerify())
	require.NoError(t, session.StartScan())
	require.NoError(t, session.StartCommit())
	require.NoError(t, session.Complete())

	require.ErrorIs(t, session.StartUpload(), ErrInvalidTransferTransition)
	require.ErrorIs(t, session.Fail("late"), ErrInvalidTransferTransition)
}

func TestTransferStateMachineAllowsTerminalOutcomes(t *testing.T) {
	for _, test := range []struct {
		name string
		run  func(*TransferSession) error
	}{
		{"fail from initiated", func(s *TransferSession) error { return s.Fail("provider_error") }},
		{"reject after scanning", func(s *TransferSession) error {
			require.NoError(t, s.StartUpload())
			require.NoError(t, s.StartVerify())
			require.NoError(t, s.StartScan())
			return s.Reject("malware")
		}},
		{"abort while paused", func(s *TransferSession) error {
			require.NoError(t, s.StartUpload())
			require.NoError(t, s.Pause())
			return s.Abort()
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			session, err := NewTransferSession(validTransferInput())
			require.NoError(t, err)
			require.NoError(t, test.run(session))
			require.Contains(t, []TransferStatus{
				TransferFailed, TransferRejected, TransferAborted,
			}, session.Status())
			require.ErrorIs(t, session.Complete(), ErrInvalidTransferTransition)
		})
	}
}

func TestTransferSessionValidatesInvariants(t *testing.T) {
	tests := []struct {
		name string
		edit func(*TransferSessionInput)
		err  error
	}{
		{"workspace required", func(v *TransferSessionInput) { v.WorkspaceID = uuid.Nil }, ErrInvalidTransfer},
		{"namespace required", func(v *TransferSessionInput) { v.NamespaceID = uuid.Nil }, ErrInvalidTransfer},
		{"creator required", func(v *TransferSessionInput) { v.CreatedBy = uuid.Nil }, ErrInvalidTransfer},
		{"name required", func(v *TransferSessionInput) { v.TargetName = "a/b" }, ErrInvalidTransferName},
		{"size bounded", func(v *TransferSessionInput) { v.ExpectedSize = maxTransferSize + 1 }, ErrInvalidTransferSize},
		{"sha is hex", func(v *TransferSessionInput) { v.ExpectedSHA256 = "not-a-sha" }, ErrInvalidTransferChecksum},
		{"protocol supported", func(v *TransferSessionInput) { v.Protocol = "ftp" }, ErrInvalidTransferProtocol},
		{"idempotency required", func(v *TransferSessionInput) { v.IdempotencyKey = "" }, ErrInvalidTransfer},
		{"expiry future", func(v *TransferSessionInput) { v.ExpiresAt = time.Now() }, ErrInvalidTransferExpiry},
		{"temporary key required", func(v *TransferSessionInput) { v.TemporaryObjectKey = "" }, ErrInvalidTransfer},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			input := validTransferInput()
			test.edit(&input)
			_, err := NewTransferSession(input)
			require.ErrorIs(t, err, test.err)
		})
	}
}

func TestTransferSessionSemanticUpdatesUseVersion(t *testing.T) {
	session, err := NewTransferSession(validTransferInput())
	require.NoError(t, err)

	require.NoError(t, session.StartUpload())
	require.Equal(t, int64(2), session.Version())
	require.ErrorIs(t, session.SetActual(1, "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef", 1), ErrTransferVersionConflict)
	require.NoError(t, session.SetActual(1, "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef", 2))
	require.Equal(t, int64(3), session.Version())
	require.Equal(t, int64(1), session.ActualSize())
	require.Equal(t, "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef", session.ActualSHA256())
}

func TestTransferPartValidatesInvariants(t *testing.T) {
	_, err := NewTransferPart(TransferPartInput{
		ID: uuid.New(), SessionID: uuid.New(), PartNumber: 0, Offset: 0, Size: 1,
	})
	require.ErrorIs(t, err, ErrInvalidTransferPart)

	part, err := NewTransferPart(TransferPartInput{
		ID: uuid.New(), SessionID: uuid.New(), PartNumber: 1, Offset: 0, Size: 1,
	})
	require.NoError(t, err)
	require.Equal(t, 1, part.PartNumber())
	require.NotPanics(t, func() { _ = errors.Is(err, ErrInvalidTransferPart) })
}
