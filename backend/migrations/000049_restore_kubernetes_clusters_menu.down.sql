UPDATE governance.menus
SET deleted_at = NOW(),
    updated_at = NOW()
WHERE path = '/plugins/kubernetes/clusters'
  AND deleted_at IS NULL;
