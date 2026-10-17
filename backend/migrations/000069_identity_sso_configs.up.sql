CREATE TABLE IF NOT EXISTS identity.sso_configs (
  provider VARCHAR(16) PRIMARY KEY,
  enabled BOOLEAN NOT NULL DEFAULT FALSE,
  config JSONB NOT NULL DEFAULT '{}',
  secret_blob TEXT NOT NULL DEFAULT '',
  tested_fingerprint VARCHAR(64) NOT NULL DEFAULT '',
  tested_at TIMESTAMPTZ NULL,
  version BIGINT NOT NULL DEFAULT 0,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  CONSTRAINT identity_sso_configs_provider_check CHECK (provider IN ('oidc', 'ldap'))
);
