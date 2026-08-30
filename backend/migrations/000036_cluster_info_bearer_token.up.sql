-- Add bearer token support for workspace cluster registration (Token credentials).
ALTER TABLE platform_admin.cluster_info
    ADD COLUMN IF NOT EXISTS bearer_token TEXT NOT NULL DEFAULT '';
