UPDATE governance.menus
SET deleted_at = NULL,
    updated_at = NOW()
WHERE path = '/plugins/kubernetes/create';
