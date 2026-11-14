package query

import (
	"bytes"
	"encoding/base64"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/domain"
)

func TestSharedCursorCodecEncryptsAndAuthenticatesPosition(t *testing.T) {
	after := uuid.NewString()
	codec, err := newSharedCursorCodec(bytes.Repeat([]byte{0x7a}, 32), bytes.NewReader(bytes.Repeat([]byte{0x42}, 12)))
	require.NoError(t, err)
	context := SharedCursorContext{
		OrganizationID: uuid.New(),
		ActorID:        uuid.New(),
		WorkspaceID:    uuid.New(),
		NamespaceID:    uuid.New(),
		Query:          " Quarterly ",
	}

	token, err := codec.Encode(after, context)
	require.NoError(t, err)
	raw, err := base64.RawURLEncoding.DecodeString(token)
	require.NoError(t, err)
	require.NotContains(t, string(raw), after)
	require.NotContains(t, string(raw), string(bytes.Repeat([]byte{0x7a}, 32)))

	decoded, err := codec.Decode(token, SharedCursorContext{
		OrganizationID: context.OrganizationID,
		ActorID:        context.ActorID,
		WorkspaceID:    context.WorkspaceID,
		NamespaceID:    context.NamespaceID,
		Query:          "quarterly",
	})
	require.NoError(t, err)
	require.Equal(t, after, decoded)

	raw[len(raw)-1] ^= 0x01
	_, err = codec.Decode(base64.RawURLEncoding.EncodeToString(raw), context)
	require.ErrorIs(t, err, domain.ErrInvalidGovernanceCursor)
}

func TestSharedCursorCodecRejectsContextReplay(t *testing.T) {
	context := SharedCursorContext{
		OrganizationID: uuid.New(),
		ActorID:        uuid.New(),
		WorkspaceID:    uuid.New(),
		NamespaceID:    uuid.New(),
		Query:          "quarterly",
	}
	codec, err := NewSharedCursorCodec("fixed-transfer-secret", bytes.NewReader(bytes.Repeat([]byte{0x24}, 12)))
	require.NoError(t, err)
	token, err := codec.Encode(uuid.NewString(), context)
	require.NoError(t, err)

	tests := []struct {
		name   string
		change func(*SharedCursorContext)
	}{
		{name: "organization", change: func(value *SharedCursorContext) { value.OrganizationID = uuid.New() }},
		{name: "actor", change: func(value *SharedCursorContext) { value.ActorID = uuid.New() }},
		{name: "workspace", change: func(value *SharedCursorContext) { value.WorkspaceID = uuid.New() }},
		{name: "namespace filter", change: func(value *SharedCursorContext) { value.NamespaceID = uuid.New() }},
		{name: "query", change: func(value *SharedCursorContext) { value.Query = "annual" }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			replay := context
			tt.change(&replay)
			_, decodeErr := codec.Decode(token, replay)
			require.ErrorIs(t, decodeErr, domain.ErrInvalidGovernanceCursor)
		})
	}
}

func TestSharedCursorCodecRejectsMalformedAndUnsupportedVersions(t *testing.T) {
	codec, err := NewSharedCursorCodec("fixed-transfer-secret", bytes.NewReader(bytes.Repeat([]byte{0x18}, 12)))
	require.NoError(t, err)
	context := SharedCursorContext{OrganizationID: uuid.New(), ActorID: uuid.New()}
	token, err := codec.Encode(uuid.NewString(), context)
	require.NoError(t, err)
	raw, err := base64.RawURLEncoding.DecodeString(token)
	require.NoError(t, err)
	raw[0]++

	for _, invalid := range []string{"not-base64!", base64.RawURLEncoding.EncodeToString(raw)} {
		_, decodeErr := codec.Decode(invalid, context)
		require.ErrorIs(t, decodeErr, domain.ErrInvalidGovernanceCursor)
	}
}

func TestSharedCursorCodecUsesFreshNoncePerToken(t *testing.T) {
	nonceBytes := append(bytes.Repeat([]byte{0x11}, 12), bytes.Repeat([]byte{0x22}, 12)...)
	codec, err := NewSharedCursorCodec("fixed-transfer-secret", bytes.NewReader(nonceBytes))
	require.NoError(t, err)
	context := SharedCursorContext{OrganizationID: uuid.New(), ActorID: uuid.New()}

	first, err := codec.Encode(uuid.NewString(), context)
	require.NoError(t, err)
	second, err := codec.Encode(uuid.NewString(), context)
	require.NoError(t, err)
	firstRaw, err := base64.RawURLEncoding.DecodeString(first)
	require.NoError(t, err)
	secondRaw, err := base64.RawURLEncoding.DecodeString(second)
	require.NoError(t, err)

	require.NotEqual(t, firstRaw[1:13], secondRaw[1:13])
}

func TestGovernanceMetadataCursorBindsOrganizationNodeAndKind(t *testing.T) {
	codec, err := NewSharedCursorCodec("metadata-cursor-secret", nil)
	require.NoError(t, err)
	context := GovernanceCursorContext{
		OrganizationID: uuid.New(), NodeID: uuid.New(), Kind: "grants",
	}
	position := GovernanceCursorPosition{
		CreatedAt: time.Date(2026, time.November, 15, 3, 0, 0, 123, time.UTC),
		ID:        uuid.New(),
	}

	token, err := codec.EncodeGovernance(position, context)
	require.NoError(t, err)
	decoded, err := codec.DecodeGovernance(token, context)
	require.NoError(t, err)
	require.Equal(t, position, decoded)

	for _, replay := range []GovernanceCursorContext{
		{OrganizationID: uuid.New(), NodeID: context.NodeID, Kind: context.Kind},
		{OrganizationID: context.OrganizationID, NodeID: uuid.New(), Kind: context.Kind},
		{OrganizationID: context.OrganizationID, NodeID: context.NodeID, Kind: "share-requests"},
	} {
		_, decodeErr := codec.DecodeGovernance(token, replay)
		require.ErrorIs(t, decodeErr, domain.ErrInvalidGovernanceCursor)
	}
	_, err = codec.DecodeGovernance("invalid", context)
	require.ErrorIs(t, err, domain.ErrInvalidGovernanceCursor)
}
