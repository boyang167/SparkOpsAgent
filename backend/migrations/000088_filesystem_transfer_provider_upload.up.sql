ALTER TABLE filesystem.transfer_sessions
    ADD COLUMN IF NOT EXISTS provider_upload_id VARCHAR(512);