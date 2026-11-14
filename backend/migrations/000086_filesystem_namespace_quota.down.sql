DO $$
BEGIN
    IF EXISTS (
        SELECT 1
        FROM filesystem.transfer_sessions session
        JOIN filesystem.namespaces namespace ON namespace.id = session.namespace_id
        WHERE namespace.namespace_type <> 'WORKSPACE'
    ) THEN
        RAISE EXCEPTION 'cannot roll back namespace quota with non-workspace transfers';
    END IF;
    IF EXISTS (
        SELECT 1
        FROM filesystem.quota_accounts account
        JOIN filesystem.namespaces namespace ON namespace.id = account.namespace_id
        WHERE namespace.namespace_type <> 'WORKSPACE'
          AND (
              account.used_bytes <> 0 OR account.reserved_bytes <> 0
              OR account.used_files <> 0 OR account.reserved_files <> 0
              OR EXISTS (
                  SELECT 1 FROM filesystem.quota_reservations reservation
                  WHERE reservation.quota_account_id = account.id
              )
          )
    ) THEN
        RAISE EXCEPTION 'cannot roll back namespace quota with non-workspace quota activity';
    END IF;
END
$$;

DROP INDEX IF EXISTS filesystem.ix_quota_reservations_namespace;
DROP INDEX IF EXISTS filesystem.ux_quota_reservations_session;

ALTER TABLE filesystem.quota_reservations
    DROP CONSTRAINT IF EXISTS fk_quota_reservations_account_namespace,
    DROP CONSTRAINT IF EXISTS fk_quota_reservations_session;

ALTER TABLE filesystem.transfer_sessions
    DROP CONSTRAINT IF EXISTS fk_transfer_sessions_namespace,
    DROP CONSTRAINT IF EXISTS uq_transfer_sessions_namespace_idempotency;

DELETE FROM filesystem.quota_accounts account
USING filesystem.namespaces namespace
WHERE namespace.id = account.namespace_id
  AND namespace.namespace_type <> 'WORKSPACE';

ALTER TABLE filesystem.quota_accounts
    DROP CONSTRAINT IF EXISTS fk_quota_accounts_namespace,
    DROP CONSTRAINT IF EXISTS uq_quota_accounts_id_namespace,
    DROP CONSTRAINT IF EXISTS uq_quota_accounts_namespace,
    ALTER COLUMN workspace_id SET NOT NULL,
    ADD CONSTRAINT quota_accounts_workspace_id_key UNIQUE (workspace_id),
    ADD CONSTRAINT uq_quota_accounts_id_workspace UNIQUE (id, workspace_id);

ALTER TABLE filesystem.transfer_sessions
    ALTER COLUMN workspace_id SET NOT NULL,
    ADD CONSTRAINT fk_transfer_sessions_namespace_workspace
        FOREIGN KEY (workspace_id, namespace_id)
        REFERENCES filesystem.namespaces(workspace_id, id);

ALTER TABLE filesystem.quota_reservations
    ALTER COLUMN workspace_id SET NOT NULL,
    ADD CONSTRAINT fk_quota_reservations_account_workspace
        FOREIGN KEY (quota_account_id, workspace_id)
        REFERENCES filesystem.quota_accounts(id, workspace_id),
    ADD CONSTRAINT fk_quota_reservations_session
        FOREIGN KEY (workspace_id, session_id)
        REFERENCES filesystem.transfer_sessions(workspace_id, id),
    DROP COLUMN IF EXISTS namespace_id;

CREATE UNIQUE INDEX ux_quota_reservations_session
    ON filesystem.quota_reservations(workspace_id, session_id)
    WHERE session_id IS NOT NULL;

ALTER TABLE filesystem.quota_accounts
    DROP COLUMN IF EXISTS namespace_id;
