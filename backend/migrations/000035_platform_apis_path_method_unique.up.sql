-- platform.apis may have been created by GORM AutoMigrate without UNIQUE(path, method).
-- Plugin enable uses ON CONFLICT (path, method) and requires this constraint.

DELETE FROM platform.apis a
USING platform.apis b
WHERE a.path = b.path
  AND a.method = b.method
  AND a.ctid < b.ctid;

CREATE UNIQUE INDEX IF NOT EXISTS ux_platform_apis_path_method
    ON platform.apis (path, method);
