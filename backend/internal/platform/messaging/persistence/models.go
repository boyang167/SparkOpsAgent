package persistence

import (
	"encoding/json"
	"time"
)

type EventOutbox struct {
	ID               string          `gorm:"column:id;type:uuid;not null;primaryKey"`
	EventType        string          `gorm:"column:event_type;size:160;not null"`
	EventVersion     int             `gorm:"column:event_version;type:int;not null"`
	AggregateType    string          `gorm:"column:aggregate_type;size:120;not null;default:''"`
	AggregateID      string          `gorm:"column:aggregate_id;size:160;not null;default:''"`
	AggregateVersion *int64          `gorm:"column:aggregate_version"`
	Payload          json.RawMessage `gorm:"column:payload;type:jsonb;not null"`
	OccurredAt       time.Time       `gorm:"column:occurred_at;not null"`
	CorrelationID    *string         `gorm:"column:correlation_id;type:uuid"`
	CausationID      *string         `gorm:"column:causation_id;type:uuid"`
	Status           string          `gorm:"column:status;size:16;not null;default:pending;check:event_outbox_status_check,status IN ('pending','publishing','published')"`
	Attempts         int             `gorm:"column:attempts;type:int;not null;default:0"`
	NextAttemptAt    time.Time       `gorm:"column:next_attempt_at;not null;default:CURRENT_TIMESTAMP"`
	LockedAt         *time.Time      `gorm:"column:locked_at"`
	LockedBy         string          `gorm:"column:locked_by;size:160;not null;default:''"`
	PublishedAt      *time.Time      `gorm:"column:published_at"`
	LastError        string          `gorm:"column:last_error;type:text;not null;default:''"`
	CreatedAt        time.Time       `gorm:"column:created_at;not null;default:CURRENT_TIMESTAMP"`
}

func (EventOutbox) TableName() string { return "platform.event_outbox" }

type EventInbox struct {
	EventID     string    `gorm:"column:event_id;type:uuid;not null;primaryKey"`
	Subscriber  string    `gorm:"column:subscriber;size:200;not null;primaryKey"`
	EventType   string    `gorm:"column:event_type;size:160;not null"`
	ProcessedAt time.Time `gorm:"column:processed_at;not null;default:CURRENT_TIMESTAMP"`
	CreatedAt   time.Time `gorm:"column:created_at;not null;default:CURRENT_TIMESTAMP"`
}

func (EventInbox) TableName() string { return "platform.event_inbox" }

type OperationOutbox struct {
	ID             string          `gorm:"column:id;type:uuid;not null;primaryKey"`
	EventID        string          `gorm:"column:event_id;type:uuid;not null"`
	Subscriber     string          `gorm:"column:subscriber;size:200;not null"`
	OperationType  string          `gorm:"column:operation_type;size:160;not null"`
	IdempotencyKey string          `gorm:"column:idempotency_key;size:240;not null"`
	Payload        json.RawMessage `gorm:"column:payload;type:jsonb;not null"`
	Status         string          `gorm:"column:status;size:16;not null;default:pending;check:operation_outbox_status_check,status IN ('pending','executing','completed','failed')"`
	Attempts       int             `gorm:"column:attempts;type:int;not null;default:0"`
	NextAttemptAt  time.Time       `gorm:"column:next_attempt_at;not null;default:CURRENT_TIMESTAMP"`
	LockedAt       *time.Time      `gorm:"column:locked_at"`
	LockedBy       string          `gorm:"column:locked_by;size:160;not null;default:''"`
	CompletedAt    *time.Time      `gorm:"column:completed_at"`
	LastError      string          `gorm:"column:last_error;type:text;not null;default:''"`
	CreatedAt      time.Time       `gorm:"column:created_at;not null;default:CURRENT_TIMESTAMP"`
}

func (OperationOutbox) TableName() string { return "platform.operation_outbox" }
