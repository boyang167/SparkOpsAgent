UPDATE governance.menus
SET deleted_at = NULL,
    updated_at = NOW()
WHERE path = '/plugins/kubernetes/clusters'
  AND deleted_at IS NOT NULL;
