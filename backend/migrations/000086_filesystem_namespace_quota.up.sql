ALTER TABLE filesystem.quota_accounts
    ADD COLUMN namespace_id UUID;

UPDATE filesystem.quota_accounts account
SET namespace_id = namespace.id
FROM filesystem.namespaces namespace
WHERE namespace.namespace_type = 'WORKSPACE'
  AND namespace.workspace_id = account.workspace_id;

INSERT INTO filesystem.quota_accounts (
    id, organization_id, namespace_id, workspace_id,
    used_bytes, reserved_bytes, used_files, reserved_files, version, updated_at
)
SELECT gen_random_uuid(), namespace.organization_id, namespace.id, namespace.workspace_id,
       0, 0, 0, 0, 0, NOW()
FROM filesystem.namespaces namespace
WHERE NOT EXISTS (
    SELECT 1 FROM filesystem.quota_accounts account
    WHERE account.namespace_id = namespace.id
);

ALTER TABLE filesystem.quota_reservations
    ADD COLUMN namespace_id UUID;

UPDATE filesystem.quota_reservations reservation
SET namespace_id = account.namespace_id
FROM filesystem.quota_accounts account
WHERE account.id = reservation.quota_account_id;

ALTER TABLE filesystem.quota_reservations
    DROP CONSTRAINT IF EXISTS fk_quota_reservations_account_workspace;

ALTER TABLE filesystem.transfer_sessions
    DROP CONSTRAINT IF EXISTS fk_transfer_sessions_namespace_workspace,
    ADD CONSTRAINT fk_transfer_sessions_namespace
        FOREIGN KEY (namespace_id) REFERENCES filesystem.namespaces(id);

ALTER TABLE filesystem.quota_accounts
    DROP CONSTRAINT IF EXISTS uq_quota_accounts_id_workspace,
    DROP CONSTRAINT IF EXISTS quota_accounts_workspace_id_key,
    ALTER COLUMN workspace_id DROP NOT NULL,
    ALTER COLUMN namespace_id SET NOT NULL,
    ADD CONSTRAINT uq_quota_accounts_namespace UNIQUE (namespace_id),
    ADD CONSTRAINT uq_quota_accounts_id_namespace UNIQUE (id, namespace_id),
    ADD CONSTRAINT fk_quota_accounts_namespace
        FOREIGN KEY (namespace_id) REFERENCES filesystem.namespaces(id);

ALTER TABLE filesystem.quota_reservations
    ALTER COLUMN namespace_id SET NOT NULL,
    ADD CONSTRAINT fk_quota_reservations_account_namespace
        FOREIGN KEY (quota_account_id, namespace_id)
        REFERENCES filesystem.quota_accounts(id, namespace_id);

CREATE INDEX IF NOT EXISTS ix_quota_reservations_namespace
    ON filesystem.quota_reservations(namespace_id);
