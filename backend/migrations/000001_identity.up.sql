CREATE SCHEMA IF NOT EXISTS identity;

CREATE TABLE IF NOT EXISTS identity.members (
    id           UUID PRIMARY KEY,
    email        VARCHAR(255) NOT NULL,
    CONSTRAINT uni_members_email UNIQUE (email),
    display_name VARCHAR(255) NOT NULL,
    password_hash TEXT NOT NULL,
    status       VARCHAR(32) NOT NULL DEFAULT 'active',
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_members_email ON identity.members(email);
