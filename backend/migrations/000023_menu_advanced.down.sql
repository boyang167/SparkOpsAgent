DROP TABLE IF EXISTS governance.role_menu_buttons;
DROP TABLE IF EXISTS governance.menu_buttons;
DROP TABLE IF EXISTS governance.menu_params;

ALTER TABLE governance.menus
    DROP COLUMN IF EXISTS active_menu,
    DROP COLUMN IF EXISTS close_tab,
    DROP COLUMN IF EXISTS is_basic,
    DROP COLUMN IF EXISTS transition_animation;
