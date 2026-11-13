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
		RequestHash:        "abcdefabcdefabcdefabcdefabcdefabcdefabcdefabcdefabcdefabcdefabcd",
		TemporaryObjectKey: "tmp/upload-1",
		ExpiresAt:          time.Now().Add(time.Hour),
		Classification:     TransferClassificationPublic,
	}
}

func TestTransferStateMachine(t *testing.T) {
	session, err := NewTransferSession(validTransferInput())
	require.NoError(t, err)

	require.NoError(t, session.StartUpload(1, time.Now()))
	require.NoError(t, session.Pause(2, time.Now()))
	require.NoError(t, session.Resume(3, time.Now()))
	require.NoError(t, session.SetActual(1024, validTransferInput().ExpectedSHA256, 4, time.Now()))
	require.NoError(t, session.StartVerify(5, time.Now()))
	require.NoError(t, session.StartScan(6, time.Now()))
	require.NoError(t, session.SetScanResult(TransferScanResult{Clean: true, CompletedAt: time.Now()}, 7, time.Now()))
	require.NoError(t, session.StartCommit(8, time.Now()))
	require.NoError(t, session.Complete(9, time.Now()))

	require.ErrorIs(t, session.StartUpload(10, time.Now()), ErrInvalidTransferTransition)
	require.ErrorIs(t, session.Fail("late", 10, time.Now()), ErrInvalidTransferTransition)
}

func TestStartCommitRequiresActualIntegrityButAllowsVerifiedZeroByteFile(t *testing.T) {
	session, err := NewTransferSession(validTransferInput())
	require.NoError(t, err)
	require.NoError(t, session.StartUpload(1, time.Now()))
	require.NoError(t, session.StartVerify(2, time.Now()))
	require.NoError(t, session.StartScan(3, time.Now()))
	require.NoError(t, session.SetScanResult(TransferScanResult{Clean: true, CompletedAt: time.Now()}, 4, time.Now()))
	require.ErrorIs(t, session.StartCommit(5, time.Now()), ErrTransferIntegrity)

	zeroInput := validTransferInput()
	zeroInput.ExpectedSize = 0
	zeroSession, err := NewTransferSession(zeroInput)
	require.NoError(t, err)
	require.NoError(t, zeroSession.StartUpload(1, time.Now()))
	require.NoError(t, zeroSession.SetActual(0, zeroInput.ExpectedSHA256, 2, time.Now()))
	require.NoError(t, zeroSession.StartVerify(3, time.Now()))
	require.NoError(t, zeroSession.StartScan(4, time.Now()))
	require.NoError(t, zeroSession.SetScanResult(TransferScanResult{Clean: true, CompletedAt: time.Now()}, 5, time.Now()))
	require.NoError(t, zeroSession.StartCommit(6, time.Now()))
}

func TestTransferAllowedTransitionMatrix(t *testing.T) {
	tests := []struct {
		name   string
		ready  func(*TransferSession)
		apply  func(*TransferSession) error
		status TransferStatus
	}{
		{"initiated to uploading", func(*TransferSession) {}, func(s *TransferSession) error {
			return s.StartUpload(1, time.Now())
		}, TransferUploading},
		{"uploading to paused", func(s *TransferSession) {
			require.NoError(t, s.StartUpload(1, time.Now()))
		}, func(s *TransferSession) error {
			return s.Pause(2, time.Now())
		}, TransferPaused},
		{"paused to uploading", func(s *TransferSession) {
			require.NoError(t, s.StartUpload(1, time.Now()))
			require.NoError(t, s.Pause(2, time.Now()))
		}, func(s *TransferSession) error {
			return s.Resume(3, time.Now())
		}, TransferUploading},
		{"uploading to verifying", func(s *TransferSession) {
			require.NoError(t, s.StartUpload(1, time.Now()))
		}, func(s *TransferSession) error {
			return s.StartVerify(2, time.Now())
		}, TransferVerifying},
		{"verifying to scanning", func(s *TransferSession) {
			require.NoError(t, s.StartUpload(1, time.Now()))
			require.NoError(t, s.StartVerify(2, time.Now()))
		}, func(s *TransferSession) error {
			return s.StartScan(3, time.Now())
		}, TransferScanning},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			session, err := NewTransferSession(validTransferInput())
			require.NoError(t, err)
			test.ready(session)
			require.NoError(t, test.apply(session))
			require.Equal(t, test.status, session.Status())
		})
	}
}

func TestTransferStateMachineAllowsTerminalOutcomes(t *testing.T) {
	for _, test := range []struct {
		name string
		run  func(*TransferSession) error
	}{
		{"fail from initiated", func(s *TransferSession) error { return s.Fail("provider_error", 1, time.Now()) }},
		{"reject after scanning", func(s *TransferSession) error {
			require.NoError(t, s.StartUpload(1, time.Now()))
			require.NoError(t, s.StartVerify(2, time.Now()))
			require.NoError(t, s.StartScan(3, time.Now()))
			return s.Reject("malware", 4, time.Now())
		}},
		{"abort while paused", func(s *TransferSession) error {
			require.NoError(t, s.StartUpload(1, time.Now()))
			require.NoError(t, s.Pause(2, time.Now()))
			return s.Abort(3, time.Now())
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			session, err := NewTransferSession(validTransferInput())
			require.NoError(t, err)
			require.NoError(t, test.run(session))
			require.Contains(t, []TransferStatus{
				TransferFailed, TransferRejected, TransferAborted,
			}, session.Status())
			require.ErrorIs(t, session.Complete(session.Version(), time.Now()), ErrInvalidTransferTransition)
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

	require.NoError(t, session.StartUpload(1, time.Now()))
	require.Equal(t, int64(2), session.Version())
	require.ErrorIs(t, session.SetActual(1, "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef", 1, time.Now()), ErrTransferVersionConflict)
	require.NoError(t, session.SetActual(1024, "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef", 2, time.Now()))
	require.Equal(t, int64(3), session.Version())
	actualSize, ok := session.ActualSize()
	require.True(t, ok)
	require.Equal(t, int64(1024), actualSize)
	require.Equal(t, "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef", session.ActualSHA256())
}

func TestTransferPartValidatesInvariants(t *testing.T) {
	_, err := NewTransferPart(TransferPartInput{
		ID: uuid.New(), SessionID: uuid.New(), PartNumber: 0, Offset: 0, Size: 1, SessionSize: 10, ETag: "etag",
	})
	require.ErrorIs(t, err, ErrInvalidTransferPart)

	part, err := NewTransferPart(TransferPartInput{
		ID: uuid.New(), SessionID: uuid.New(), PartNumber: 1, Offset: 0, Size: 1, SessionSize: 10, ETag: "etag",
	})
	require.NoError(t, err)
	require.Equal(t, 1, part.PartNumber())
	require.NotPanics(t, func() { _ = errors.Is(err, ErrInvalidTransferPart) })
}

func TestTransferSessionRequiresExactCASAndExpiry(t *testing.T) {
	session, err := NewTransferSession(validTransferInput())
	require.NoError(t, err)
	require.ErrorIs(t, session.StartUpload(2, time.Now()), ErrTransferVersionConflict)
	require.ErrorIs(t, session.StartUpload(1, session.ExpiresAt().Add(time.Second)), ErrTransferExpired)
	require.Equal(t, TransferInitiated, session.Status())
}

func TestRestrictedTransferFailsClosedUntilCleanScan(t *testing.T) {
	input := validTransferInput()
	input.Classification = TransferClassificationRestricted
	session, err := NewTransferSession(input)
	require.NoError(t, err)

	require.NoError(t, session.StartUpload(1, time.Now()))
	require.NoError(t, session.SetActual(1024, input.ExpectedSHA256, 2, time.Now()))
	require.NoError(t, session.StartVerify(3, time.Now()))
	require.NoError(t, session.StartScan(4, time.Now()))
	require.ErrorIs(t, session.StartCommit(5, time.Now()), ErrTransferScanRequired)
	require.ErrorIs(t, session.Reject("threat", 5, time.Now()), nil)
}

func TestUncleanScanCanOnlyReject(t *testing.T) {
	session, err := NewTransferSession(validTransferInput())
	require.NoError(t, err)
	require.NoError(t, session.StartUpload(1, time.Now()))
	require.NoError(t, session.SetActual(1024, session.ExpectedSHA256(), 2, time.Now()))
	require.NoError(t, session.StartVerify(3, time.Now()))
	require.NoError(t, session.StartScan(4, time.Now()))
	require.NoError(t, session.SetScanResult(TransferScanResult{Clean: false, ThreatName: "EICAR", CompletedAt: time.Now()}, 5, time.Now()))
	require.ErrorIs(t, session.StartCommit(6, time.Now()), ErrTransferScanRequired)
	require.NoError(t, session.Reject("malware", 6, time.Now()))
	require.ErrorIs(t, session.Reject("again", 7, time.Now()), ErrInvalidTransferTransition)
}

func TestTransferIntegrityMustMatchExpected(t *testing.T) {
	session, err := NewTransferSession(validTransferInput())
	require.NoError(t, err)
	require.NoError(t, session.StartUpload(1, time.Now()))
	require.ErrorIs(t, session.SetActual(1, "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef", 2, time.Now()), ErrTransferIntegrity)
	require.ErrorIs(t, session.SetActual(1024, "abcdefabcdefabcdefabcdefabcdefabcdefabcdefabcdefabcdefabcdefabcd", 2, time.Now()), ErrTransferIntegrity)
}

func TestTransferPartRejectsBoundaries(t *testing.T) {
	base := TransferPartInput{ID: uuid.New(), SessionID: uuid.New(), PartNumber: 1, Offset: 0, Size: 1, SessionSize: 10, ETag: "etag"}
	for name, edit := range map[string]func(*TransferPartInput){
		"zero size":    func(v *TransferPartInput) { v.Size = 0 },
		"empty etag":   func(v *TransferPartInput) { v.ETag = "" },
		"past session": func(v *TransferPartInput) { v.Offset, v.Size = 9, 2 },
		"overflow":     func(v *TransferPartInput) { v.Offset, v.Size = int64(^uint64(0)>>1), 1 },
	} {
		t.Run(name, func(t *testing.T) {
			input := base
			edit(&input)
			_, err := NewTransferPart(input)
			require.ErrorIs(t, err, ErrInvalidTransferPart)
		})
	}
}

func TestEveryTerminalTransferStateCannotMigrate(t *testing.T) {
	builders := []func(*TransferSession) error{
		func(s *TransferSession) error { return s.Fail("error", 1, time.Now()) },
		func(s *TransferSession) error {
			require.NoError(t, s.StartUpload(1, time.Now()))
			require.NoError(t, s.StartVerify(2, time.Now()))
			require.NoError(t, s.StartScan(3, time.Now()))
			return s.Reject("threat", 4, time.Now())
		},
		func(s *TransferSession) error { return s.Abort(1, time.Now()) },
		func(s *TransferSession) error { return s.Expire(1, s.ExpiresAt().Add(time.Second)) },
	}
	for _, build := range builders {
		session, err := NewTransferSession(validTransferInput())
		require.NoError(t, err)
		require.NoError(t, build(session))
		require.ErrorIs(t, session.StartUpload(session.Version(), time.Now()), ErrInvalidTransferTransition)
		require.ErrorIs(t, session.Pause(session.Version(), time.Now()), ErrInvalidTransferTransition)
		require.ErrorIs(t, session.StartVerify(session.Version(), time.Now()), ErrInvalidTransferTransition)
		require.ErrorIs(t, session.StartScan(session.Version(), time.Now()), ErrInvalidTransferTransition)
		require.ErrorIs(t, session.StartCommit(session.Version(), time.Now()), ErrInvalidTransferTransition)
		require.ErrorIs(t, session.Complete(session.Version(), time.Now()), ErrInvalidTransferTransition)
		require.ErrorIs(t, session.Fail("again", session.Version(), time.Now()), ErrInvalidTransferTransition)
		require.ErrorIs(t, session.Reject("again", session.Version(), time.Now()), ErrInvalidTransferTransition)
		require.ErrorIs(t, session.Abort(session.Version(), time.Now()), ErrInvalidTransferTransition)
	}
}
