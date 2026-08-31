package terminal

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"sync"
	"time"

	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

const (
	WebsocketPath       = "/ws/plugins/kubernetes/terminal"
	TicketTTL           = 30 * time.Second
	MaxTicketsPerMember = 5
)

type TicketClaims struct {
	WorkspaceID kernel.WorkspaceID
	MemberID    kernel.MemberID
	ClusterID   kernel.ClusterInfoID
	Namespace   string
	Pod         string
	Container   string
	Cols        uint16
	Rows        uint16
}

type IssuedTicket struct {
	Ticket        string    `json:"ticket"`
	ExpiresAt     time.Time `json:"expires_at"`
	WebsocketPath string    `json:"websocket_path"`
}

type ticketRecord struct {
	claims    TicketClaims
	expiresAt time.Time
}

type TicketStore struct {
	now     func() time.Time
	mu      sync.Mutex
	tickets map[string]ticketRecord
}

func NewTicketStore(now func() time.Time) *TicketStore {
	if now == nil {
		now = time.Now
	}
	return &TicketStore{now: now, tickets: make(map[string]ticketRecord)}
}

func (s *TicketStore) Issue(claims TicketClaims) (IssuedTicket, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := s.now()
	s.purgeLocked(now)
	count := 0
	for _, record := range s.tickets {
		if record.claims.MemberID == claims.MemberID {
			count++
		}
	}
	if count >= MaxTicketsPerMember {
		return IssuedTicket{}, errors.New("too many unused terminal tickets")
	}

	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return IssuedTicket{}, err
	}
	ticket := hex.EncodeToString(raw)
	expiresAt := now.Add(TicketTTL)
	s.tickets[ticket] = ticketRecord{claims: claims, expiresAt: expiresAt}
	return IssuedTicket{
		Ticket:        ticket,
		ExpiresAt:     expiresAt,
		WebsocketPath: WebsocketPath,
	}, nil
}

func (s *TicketStore) Consume(ticket string) (TicketClaims, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := s.now()
	s.purgeLocked(now)
	record, ok := s.tickets[ticket]
	if !ok {
		return TicketClaims{}, false
	}
	delete(s.tickets, ticket)
	if !record.expiresAt.After(now) {
		return TicketClaims{}, false
	}
	return record.claims, true
}

func (s *TicketStore) Peek(ticket string) (TicketClaims, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.purgeLocked(s.now())
	record, ok := s.tickets[ticket]
	if !ok {
		return TicketClaims{}, false
	}
	return record.claims, true
}

func (s *TicketStore) purgeLocked(now time.Time) {
	for ticket, record := range s.tickets {
		if !record.expiresAt.After(now) {
			delete(s.tickets, ticket)
		}
	}
}
