-- Remove legacy envFrom configMapRef(bionet-env-config) from Deployment templates.
UPDATE workspace_app.templates
SET detail = regexp_replace(
        detail,
        E'\n        envFrom:\n        - configMapRef:\n            name: bionet-env-config',
        '',
        'g'
    ),
    updated_at = NOW()
WHERE deleted_at IS NULL
  AND detail LIKE '%bionet-env-config%';
