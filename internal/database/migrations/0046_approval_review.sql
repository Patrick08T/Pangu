ALTER TABLE pangu.approvals ADD COLUMN round integer NOT NULL DEFAULT 1 CHECK(round>0);
ALTER TABLE pangu.approvals ADD COLUMN previous_id uuid REFERENCES pangu.approvals(id);
ALTER TABLE pangu.approvals ADD COLUMN items jsonb NOT NULL DEFAULT '[]';
ALTER TABLE pangu.approvals DROP CONSTRAINT approvals_run_id_node_id_key;
ALTER TABLE pangu.approvals ADD UNIQUE(run_id,node_id,round);
ALTER TABLE pangu.approvals DROP CONSTRAINT approvals_status_check;
ALTER TABLE pangu.approvals ADD CONSTRAINT approvals_status_check
 CHECK(status IN ('waiting','approved','rejected','revoked','consumed','expired','changes_requested'));
DROP INDEX pangu.approvals_outbox;
CREATE INDEX approvals_outbox ON pangu.approvals(namespace) WHERE signalled_at IS NULL AND status IN ('approved','rejected','revoked','changes_requested');
CREATE INDEX approvals_project ON pangu.approvals(namespace,project_id,created_at DESC,id);

CREATE FUNCTION pangu.purge_approval_items() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
 IF NEW.subject IS NULL THEN NEW.items='[]'::jsonb; END IF;
 RETURN NEW;
END $$;
CREATE TRIGGER purge_approval_items BEFORE UPDATE OF subject ON pangu.approvals
 FOR EACH ROW EXECUTE FUNCTION pangu.purge_approval_items();
