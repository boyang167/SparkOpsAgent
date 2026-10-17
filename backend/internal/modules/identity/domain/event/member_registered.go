package event

import (
	"github.com/sparkops/sparkops-agent/backend/internal/shared/events"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

const MemberRegisteredName = "identity.MemberRegistered"

type MemberRegistered struct {
	events.BaseEvent
	MemberID kernel.MemberID
	Email    string
}

func NewMemberRegistered(memberID kernel.MemberID, email string) MemberRegistered {
	return MemberRegistered{
		BaseEvent: events.NewBaseEvent(MemberRegisteredName),
		MemberID:  memberID,
		Email:     email,
	}
}

func Register(registry *events.Registry) error {
	return events.RegisterJSON[MemberRegistered](
		registry,
		MemberRegisteredName,
		1,
		func(ev MemberRegistered) events.AggregateRef {
			return events.AggregateRef{Type: "member", ID: ev.MemberID.String()}
		},
	)
}
