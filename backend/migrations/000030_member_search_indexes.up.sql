-- Prefix search indexes for member invite autocomplete (100k+ users).
CREATE INDEX IF NOT EXISTS idx_members_email_lower_active
    ON identity.members (lower(email) varchar_pattern_ops)
    WHERE deleted_at IS NULL AND status = 'active';

CREATE INDEX IF NOT EXISTS idx_members_display_name_lower_active
    ON identity.members (lower(display_name) varchar_pattern_ops)
    WHERE deleted_at IS NULL AND status = 'active';
