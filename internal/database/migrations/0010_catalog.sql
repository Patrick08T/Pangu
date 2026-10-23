CREATE TABLE pangu.catalog_objects (
    id uuid PRIMARY KEY,
    namespace text NOT NULL,
    kind text NOT NULL CHECK (kind IN ('template','agent','workflow','connection')),
    project_id uuid REFERENCES pangu.projects(id),
    name text NOT NULL,
    revision integer NOT NULL DEFAULT 1 CHECK (revision > 0),
    archived boolean NOT NULL DEFAULT false,
    spec jsonb NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CHECK ((kind='template') = (project_id IS NULL))
);
CREATE INDEX catalog_objects_scope ON pangu.catalog_objects(namespace,kind,project_id,created_at,id);
CREATE TABLE pangu.catalog_versions (
    id uuid PRIMARY KEY,
    object_id uuid NOT NULL REFERENCES pangu.catalog_objects(id),
    number integer NOT NULL CHECK (number > 0),
    source_revision integer NOT NULL,
    spec jsonb NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE(object_id,number),
    UNIQUE(object_id,source_revision)
);
CREATE FUNCTION pangu.protect_catalog_identity() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    IF NEW.id<>OLD.id OR NEW.namespace<>OLD.namespace OR NEW.kind<>OLD.kind OR NEW.project_id IS DISTINCT FROM OLD.project_id THEN
        RAISE EXCEPTION '配置身份和绑定项目不可修改';
    END IF;
    RETURN NEW;
END $$;
CREATE TRIGGER protect_catalog_identity BEFORE UPDATE ON pangu.catalog_objects FOR EACH ROW EXECUTE FUNCTION pangu.protect_catalog_identity();
CREATE FUNCTION pangu.protect_catalog_version() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    RAISE EXCEPTION '已发布配置版本不可修改或删除';
END $$;
CREATE TRIGGER protect_catalog_version BEFORE UPDATE OR DELETE ON pangu.catalog_versions FOR EACH ROW EXECUTE FUNCTION pangu.protect_catalog_version();
