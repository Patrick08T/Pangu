ALTER TABLE pangu.evidence_repairs ALTER COLUMN body DROP NOT NULL;

CREATE FUNCTION pangu.purge_evidence_repairs() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
 IF NEW.subject IS NULL THEN
  UPDATE pangu.evidence_repairs SET body=NULL WHERE approval_id=NEW.id;
 END IF;
 RETURN NEW;
END $$;
CREATE TRIGGER purge_evidence_repairs AFTER UPDATE OF subject ON pangu.approvals
 FOR EACH ROW EXECUTE FUNCTION pangu.purge_evidence_repairs();
