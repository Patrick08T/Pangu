DROP TRIGGER protect_catalog_identity ON pangu.catalog_objects;
ALTER TABLE pangu.catalog_objects DROP CONSTRAINT catalog_objects_check;
ALTER TABLE pangu.catalog_objects DROP CONSTRAINT catalog_objects_kind_check;
UPDATE pangu.catalog_objects SET kind=CASE WHEN spec ? 'flow' THEN 'workflow' ELSE 'agent' END WHERE kind='template';
ALTER TABLE pangu.catalog_objects ADD CONSTRAINT catalog_objects_kind_check CHECK (kind IN ('agent','workflow','connection'));
DROP INDEX pangu.catalog_objects_scope;
ALTER TABLE pangu.catalog_objects DROP COLUMN project_id;
CREATE INDEX catalog_objects_scope ON pangu.catalog_objects(namespace,kind,created_at,id);
CREATE OR REPLACE FUNCTION pangu.protect_catalog_identity() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    IF NEW.id<>OLD.id OR NEW.namespace<>OLD.namespace OR NEW.kind<>OLD.kind THEN
        RAISE EXCEPTION '配置身份不可修改';
    END IF;
    RETURN NEW;
END $$;
CREATE TRIGGER protect_catalog_identity BEFORE UPDATE ON pangu.catalog_objects FOR EACH ROW EXECUTE FUNCTION pangu.protect_catalog_identity();
