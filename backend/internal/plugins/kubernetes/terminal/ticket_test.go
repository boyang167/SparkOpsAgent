package terminal

import (
	"testing"
	"time"

	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
	"github.com/stretchr/testify/require"
)

func TestTicketStoreIssuesOpaqueOneTimeTicket(t *testing.T) {
	now := time.Date(2026, 8, 31, 14, 59, 0, 0, time.UTC)
	store := NewTicketStore(func() time.Time { return now })
	claims := TicketClaims{
		WorkspaceID: kernel.NewWorkspaceID(),
		MemberID:    kernel.NewMemberID(),
		ClusterID:   kernel.NewClusterInfoID(),
		Namespace:   "kube-system",
		Pod:         "kube-proxy-xfkvs",
		Container:   "kube-proxy",
		Cols:        120,
		Rows:        36,
	}

	issued, err := store.Issue(claims)
	require.NoError(t, err)
	require.Len(t, issued.Ticket, 64)
	require.NotContains(t, issued.Ticket, claims.Pod)
	require.Equal(t, WebsocketPath, issued.WebsocketPath)
	require.Equal(t, now.Add(TicketTTL), issued.ExpiresAt)

	peeked, ok := store.Peek(issued.Ticket)
	require.True(t, ok)
	require.Equal(t, claims, peeked)
	_, ok = store.Peek(issued.Ticket)
	require.True(t, ok)

	got, ok := store.Consume(issued.Ticket)
	require.True(t, ok)
	require.Equal(t, claims, got)
	_, ok = store.Consume(issued.Ticket)
	require.False(t, ok)
}

func TestTicketStoreRejectsExpiredAndOverQuota(t *testing.T) {
	now := time.Date(2026, 8, 31, 14, 59, 0, 0, time.UTC)
	store := NewTicketStore(func() time.Time { return now })
	member := kernel.NewMemberID()
	base := TicketClaims{
		WorkspaceID: kernel.NewWorkspaceID(),
		MemberID:    member,
		ClusterID:   kernel.NewClusterInfoID(),
		Namespace:   "ns",
		Pod:         "pod",
		Container:   "c",
		Cols:        80,
		Rows:        24,
	}

	expired, err := store.Issue(base)
	require.NoError(t, err)
	now = now.Add(TicketTTL + time.Second)
	_, ok := store.Consume(expired.Ticket)
	require.False(t, ok)

	for i := 0; i < MaxTicketsPerMember; i++ {
		_, err := store.Issue(base)
		require.NoError(t, err)
	}
	_, err = store.Issue(base)
	require.Error(t, err)
}
