INSERT INTO plugin.plugins (
    id, version, display_name, manifest_yaml, package_digest, installed_files,
    status, pr_url, created_by, created_at, updated_at
) VALUES (
    'example',
    '0.1.0',
    'Example Plugin',
    E'id: example\nversion: "0.1.0"\ndisplay_name: Example Plugin\nroute_prefix: /api/v1/plugins/example\nentitlement: plugin:example\nmenus:\n  - title: Example\n    path: /plugins/example\n    component: plugins/example/page\napis:\n  - method: GET\n    path: /api/v1/plugins/example/ping\nfrontend:\n  pages:\n    - app/(platform)/plugins/example/page.tsx\n',
    '',
    '["backend/internal/plugins/example/plugin.go","backend/internal/plugins/example/plugin.yaml","fronted/apps/web/app/(platform)/plugins/example/page.tsx"]'::jsonb,
    'deployed',
    '',
    '00000000-0000-0000-0000-000000000000',
    now(),
    now()
)
ON CONFLICT (id) DO NOTHING;
