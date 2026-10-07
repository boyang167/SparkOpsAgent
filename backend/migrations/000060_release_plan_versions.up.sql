-- Release plan version history (Phase R4)

ALTER TABLE product.release_plans
    ADD COLUMN IF NOT EXISTS version INT NOT NULL DEFAULT 1;

CREATE TABLE IF NOT EXISTS product.release_plan_versions (
    release_plan_id   UUID NOT NULL REFERENCES product.release_plans(id) ON DELETE CASCADE,
    version           INT NOT NULL,
    previous_version  INT NOT NULL DEFAULT 0,
    actor_id          UUID NOT NULL,
    verb              VARCHAR(64) NOT NULL,
    section_key       VARCHAR(64) NOT NULL DEFAULT 'plan',
    section_name      VARCHAR(255),
    snapshot          JSONB NOT NULL,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (release_plan_id, version)
);

CREATE INDEX IF NOT EXISTS idx_release_plan_versions_plan
    ON product.release_plan_versions(release_plan_id, version DESC);
