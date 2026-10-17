CREATE SCHEMA IF NOT EXISTS platform;

CREATE TABLE IF NOT EXISTS platform.event_outbox (
  id UUID PRIMARY KEY,
  event_type VARCHAR(160) NOT NULL,
  event_version INT NOT NULL,
  aggregate_type VARCHAR(120) NOT NULL DEFAULT '',
  aggregate_id VARCHAR(160) NOT NULL DEFAULT '',
  aggregate_version BIGINT NULL,
  payload JSONB NOT NULL,
  occurred_at TIMESTAMPTZ NOT NULL,
  correlation_id UUID NULL,
  causation_id UUID NULL,
  status VARCHAR(16) NOT NULL DEFAULT 'pending',
  attempts INT NOT NULL DEFAULT 0,
  next_attempt_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  locked_at TIMESTAMPTZ NULL,
  locked_by VARCHAR(160) NOT NULL DEFAULT '',
  published_at TIMESTAMPTZ NULL,
  last_error TEXT NOT NULL DEFAULT '',
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  CONSTRAINT event_outbox_status_check CHECK (status IN ('pending','publishing','published'))
);
CREATE INDEX IF NOT EXISTS ix_event_outbox_claim
  ON platform.event_outbox(status, next_attempt_at, created_at);

CREATE TABLE IF NOT EXISTS platform.event_inbox (
  event_id UUID NOT NULL,
  subscriber VARCHAR(200) NOT NULL,
  event_type VARCHAR(160) NOT NULL,
  processed_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  PRIMARY KEY (event_id, subscriber)
);

CREATE TABLE IF NOT EXISTS platform.operation_outbox (
  id UUID PRIMARY KEY,
  event_id UUID NOT NULL,
  subscriber VARCHAR(200) NOT NULL,
  operation_type VARCHAR(160) NOT NULL,
  idempotency_key VARCHAR(240) NOT NULL,
  payload JSONB NOT NULL,
  status VARCHAR(16) NOT NULL DEFAULT 'pending',
  attempts INT NOT NULL DEFAULT 0,
  next_attempt_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  locked_at TIMESTAMPTZ NULL,
  locked_by VARCHAR(160) NOT NULL DEFAULT '',
  completed_at TIMESTAMPTZ NULL,
  last_error TEXT NOT NULL DEFAULT '',
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  CONSTRAINT operation_outbox_status_check CHECK (status IN ('pending','executing','completed','failed'))
);
CREATE UNIQUE INDEX IF NOT EXISTS ux_operation_outbox_idempotency_key
  ON platform.operation_outbox(idempotency_key);
CREATE INDEX IF NOT EXISTS ix_operation_outbox_claim
  ON platform.operation_outbox(status, next_attempt_at, created_at);
