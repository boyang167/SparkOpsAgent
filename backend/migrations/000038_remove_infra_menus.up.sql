UPDATE governance.menus
SET deleted_at = NOW(),
    updated_at = NOW()
WHERE path IN (
    '/config/configmaps',
    '/config/storage',
    '/discovery/services',
    '/security/ingress',
    '/security/virtualservices',
    '/config/consul'
  )
  AND deleted_at IS NULL;

DROP TABLE IF EXISTS platform.consul_kv;
