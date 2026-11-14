CREATE INDEX ix_transfer_sessions_workspace_created
    ON filesystem.transfer_sessions(workspace_id, created_at DESC, id DESC);

CREATE INDEX ix_transfer_sessions_workspace_status_created
    ON filesystem.transfer_sessions(workspace_id, status, created_at DESC, id DESC);
