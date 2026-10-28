ALTER TABLE pangu.submissions ADD COLUMN task_id uuid;
WITH RECURSIVE lineage AS (
 SELECT id,namespace,project_id,id AS ancestor,snapshot->>'source_run_id' AS source,ARRAY[id] AS visited,0 AS depth FROM pangu.submissions
 UNION ALL
 SELECT c.id,c.namespace,c.project_id,p.id,p.snapshot->>'source_run_id',c.visited||p.id,c.depth+1
 FROM lineage c JOIN pangu.submissions p ON p.run_id::text=c.source AND p.namespace=c.namespace AND p.project_id=c.project_id
 WHERE NOT p.id=ANY(c.visited)
), roots AS (SELECT DISTINCT ON (id) id,ancestor FROM lineage ORDER BY id,depth DESC)
UPDATE pangu.submissions s SET task_id=r.ancestor FROM roots r WHERE s.id=r.id;
ALTER TABLE pangu.submissions ALTER COLUMN task_id SET NOT NULL;
CREATE INDEX submissions_task ON pangu.submissions(namespace,project_id,task_id,created_at,id);
CREATE FUNCTION pangu.assign_task_identity() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
 IF coalesce(NEW.snapshot->>'source_run_id','')='' THEN NEW.task_id:=NEW.id;
 ELSE
  SELECT task_id INTO NEW.task_id FROM pangu.submissions
   WHERE namespace=NEW.namespace AND project_id=NEW.project_id AND run_id::text=NEW.snapshot->>'source_run_id';
  IF NEW.task_id IS NULL THEN RAISE EXCEPTION '来源运行不存在或不属于当前项目'; END IF;
 END IF;
 RETURN NEW;
END $$;
CREATE TRIGGER assign_task_identity BEFORE INSERT ON pangu.submissions FOR EACH ROW EXECUTE FUNCTION pangu.assign_task_identity();
CREATE FUNCTION pangu.protect_task_identity() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
 IF NEW.task_id<>OLD.task_id THEN RAISE EXCEPTION '业务任务身份不可修改'; END IF;
 RETURN NEW;
END $$;
CREATE TRIGGER protect_task_identity BEFORE UPDATE OF task_id ON pangu.submissions FOR EACH ROW EXECUTE FUNCTION pangu.protect_task_identity();
