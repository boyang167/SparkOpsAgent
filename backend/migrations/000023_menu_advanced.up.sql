ALTER TABLE governance.menus
    ADD COLUMN IF NOT EXISTS active_menu VARCHAR(128) NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS close_tab BOOLEAN NOT NULL DEFAULT FALSE,
    ADD COLUMN IF NOT EXISTS is_basic BOOLEAN NOT NULL DEFAULT FALSE,
    ADD COLUMN IF NOT EXISTS transition_animation VARCHAR(32) NOT NULL DEFAULT 'global';

CREATE TABLE IF NOT EXISTS governance.menu_params (
    id         UUID PRIMARY KEY,
    menu_id    UUID NOT NULL,
    param_type VARCHAR(16) NOT NULL DEFAULT 'query',
    param_key  VARCHAR(128) NOT NULL,
    param_value VARCHAR(512) NOT NULL DEFAULT '',
    sort       INT NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_menu_params_menu ON governance.menu_params(menu_id);

CREATE TABLE IF NOT EXISTS governance.menu_buttons (
    id         UUID PRIMARY KEY,
    menu_id    UUID NOT NULL,
    name       VARCHAR(128) NOT NULL,
    code       VARCHAR(64) NOT NULL,
    remark     VARCHAR(255) NOT NULL DEFAULT '',
    sort       INT NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(menu_id, code)
);

CREATE INDEX IF NOT EXISTS idx_menu_buttons_menu ON governance.menu_buttons(menu_id);

CREATE TABLE IF NOT EXISTS governance.role_menu_buttons (
    role_id   UUID NOT NULL,
    button_id UUID NOT NULL,
    PRIMARY KEY(role_id, button_id)
);

CREATE INDEX IF NOT EXISTS idx_role_menu_buttons_button ON governance.role_menu_buttons(button_id);
