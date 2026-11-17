-- 自动版本身份保持不可变；私有正文与可变生命周期单独存储。
ALTER TABLE pangu.catalog_versions
    ALTER COLUMN source_revision DROP NOT NULL,
    ADD COLUMN source_kind text NOT NULL DEFAULT 'manual' CHECK (source_kind IN ('manual','auto')),
    ADD COLUMN source_experiment uuid REFERENCES pangu.evaluation_experiments(id),
    ADD COLUMN project_id uuid REFERENCES pangu.projects(id),
    ADD COLUMN public_baseline uuid REFERENCES pangu.catalog_versions(id),
    ADD COLUMN project_previous uuid REFERENCES pangu.catalog_versions(id),
    ADD COLUMN content_hash text,
    ADD CONSTRAINT catalog_version_source CHECK (
      (source_kind='manual' AND source_revision IS NOT NULL AND source_experiment IS NULL AND project_id IS NULL AND public_baseline IS NULL AND project_previous IS NULL)
      OR (source_kind='auto' AND source_revision IS NULL AND source_experiment IS NOT NULL AND project_id IS NOT NULL AND public_baseline IS NOT NULL AND content_hash IS NOT NULL AND content_hash ~ '^[0-9a-f]{64}$' AND spec='{}'::jsonb));
ALTER TABLE pangu.catalog_versions DROP CONSTRAINT catalog_versions_object_id_source_revision_key;
CREATE UNIQUE INDEX catalog_manual_source ON pangu.catalog_versions(object_id,source_revision) WHERE source_kind='manual';
CREATE UNIQUE INDEX catalog_auto_source ON pangu.catalog_versions(object_id,source_experiment) WHERE source_kind='auto';
CREATE INDEX catalog_project_versions ON pangu.catalog_versions(object_id,project_id,number DESC);

CREATE TABLE pangu.catalog_version_state (
    version_id uuid PRIMARY KEY REFERENCES pangu.catalog_versions(id),
    state text NOT NULL CHECK (state IN ('observing','stable','revoked')),
    revision integer NOT NULL DEFAULT 1 CHECK (revision>0),
    updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE TABLE pangu.catalog_version_bodies (
    version_id uuid PRIMARY KEY REFERENCES pangu.catalog_versions(id),
    spec jsonb,
    cleared_at timestamptz,
    CHECK ((spec IS NULL) = (cleared_at IS NOT NULL))
);
CREATE TABLE pangu.catalog_experiment_candidates (
    experiment_id uuid PRIMARY KEY REFERENCES pangu.evaluation_experiments(id),
    payload jsonb,
    payload_hash text NOT NULL CHECK (payload_hash ~ '^[0-9a-f]{64}$'),
    candidate_hash text NOT NULL CHECK (candidate_hash ~ '^[0-9a-f]{64}$'),
    cleared_at timestamptz,
    CHECK ((payload IS NULL) = (cleared_at IS NOT NULL))
);

CREATE FUNCTION pangu.protect_catalog_private_body() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    IF TG_OP='DELETE' THEN RAISE EXCEPTION '私有正文只能清理，不能删除身份'; END IF;
    IF TG_TABLE_NAME='catalog_version_bodies' THEN
        IF NEW.version_id<>OLD.version_id OR OLD.spec IS NULL OR NEW.spec IS NOT NULL OR NEW.cleared_at IS NULL
          OR NOT EXISTS (SELECT 1 FROM pangu.catalog_version_state WHERE version_id=OLD.version_id AND state='revoked') THEN
            RAISE EXCEPTION '私有版本正文不可修改或恢复';
        END IF;
    ELSE
        IF NEW.experiment_id<>OLD.experiment_id OR NEW.payload_hash<>OLD.payload_hash OR NEW.candidate_hash<>OLD.candidate_hash
          OR OLD.payload IS NULL OR NEW.payload IS NOT NULL OR NEW.cleared_at IS NULL
          OR NOT EXISTS (SELECT 1 FROM pangu.evaluation_experiments WHERE id=OLD.experiment_id AND status='invalidated') THEN
            RAISE EXCEPTION '实验候选正文不可修改或恢复';
        END IF;
    END IF;
    RETURN NEW;
END $$;
CREATE TRIGGER protect_catalog_private_body BEFORE UPDATE OR DELETE ON pangu.catalog_version_bodies
    FOR EACH ROW EXECUTE FUNCTION pangu.protect_catalog_private_body();
CREATE TRIGGER protect_catalog_candidate_body BEFORE UPDATE OR DELETE ON pangu.catalog_experiment_candidates
    FOR EACH ROW EXECUTE FUNCTION pangu.protect_catalog_private_body();

CREATE FUNCTION pangu.protect_catalog_lifecycle() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    IF TG_OP='DELETE' THEN RAISE EXCEPTION '版本生命周期不可删除'; END IF;
    IF NEW.version_id<>OLD.version_id OR NEW.revision<>OLD.revision+1 OR OLD.state='revoked'
      OR NOT ((OLD.state='observing' AND NEW.state IN ('stable','revoked')) OR (OLD.state='stable' AND NEW.state='revoked')) THEN
        RAISE EXCEPTION '版本生命周期转换无效';
    END IF;
    RETURN NEW;
END $$;
CREATE TRIGGER protect_catalog_lifecycle BEFORE UPDATE OR DELETE ON pangu.catalog_version_state
    FOR EACH ROW EXECUTE FUNCTION pangu.protect_catalog_lifecycle();
