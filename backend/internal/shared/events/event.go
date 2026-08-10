package events

import "time"

type Event interface {
	EventName() string
	OccurredAt() time.Time
}

type BaseEvent struct {
	Name string
	At   time.Time
}

func (e BaseEvent) EventName() string     { return e.Name }
func (e BaseEvent) OccurredAt() time.Time { return e.At }

func NewBaseEvent(name string) BaseEvent {
	return BaseEvent{Name: name, At: time.Now().UTC()}
}
