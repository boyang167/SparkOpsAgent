package events

import (
	"encoding/json"
	"time"
)

type Envelope struct {
	ID               string          `json:"id"`
	Type             string          `json:"type"`
	Version          int             `json:"version"`
	AggregateType    string          `json:"aggregate_type,omitempty"`
	AggregateID      string          `json:"aggregate_id,omitempty"`
	AggregateVersion *int64          `json:"aggregate_version,omitempty"`
	OccurredAt       time.Time       `json:"occurred_at"`
	CorrelationID    string          `json:"correlation_id,omitempty"`
	CausationID      string          `json:"causation_id,omitempty"`
	Payload          json.RawMessage `json:"payload"`
}

type AggregateRef struct {
	Type    string
	ID      string
	Version *int64
}
