ALTER TABLE pangu.approvals ADD COLUMN approval_generation integer NOT NULL DEFAULT 1 CHECK(approval_generation>0);
ALTER TABLE pangu.approvals ADD COLUMN invalidation_reason text NOT NULL DEFAULT '';
ALTER TABLE pangu.approvals ADD COLUMN invalidated_at timestamptz;
ALTER TABLE pangu.approvals DROP CONSTRAINT approvals_run_id_node_id_round_key;
ALTER TABLE pangu.approvals ADD UNIQUE(run_id,node_id,approval_generation,round);
ALTER TABLE pangu.approvals DROP CONSTRAINT approvals_status_check;
ALTER TABLE pangu.approvals ADD CONSTRAINT approvals_status_check
 CHECK(status IN ('waiting','approved','rejected','revoked','consumed','expired','changes_requested','invalidated'));
DROP INDEX pangu.approvals_outbox;
CREATE INDEX approvals_outbox ON pangu.approvals(namespace) WHERE signalled_at IS NULL AND (status IN ('approved','rejected','revoked','changes_requested','invalidated') OR invalidated_at IS NOT NULL);

CREATE TABLE pangu.approval_rebuild_intents (
 id uuid PRIMARY KEY,
 namespace text NOT NULL,
 request_id uuid NOT NULL REFERENCES pangu.submissions(id) ON DELETE CASCADE,
 run_id uuid NOT NULL,
 node_id text NOT NULL,
 from_generation integer NOT NULL CHECK(from_generation>0),
 to_generation integer NOT NULL CHECK(to_generation=from_generation+1),
 reason text NOT NULL,
 requires_stop boolean NOT NULL,
 created_at timestamptz NOT NULL DEFAULT now(),
 ready_at timestamptz,
 UNIQUE(run_id,node_id,from_generation)
);
CREATE INDEX approval_rebuild_intents_recovery ON pangu.approval_rebuild_intents(namespace,run_id,created_at,id);

CREATE TABLE pangu.approval_rebuild_successors (
 intent_id uuid PRIMARY KEY REFERENCES pangu.approval_rebuild_intents(id) ON DELETE CASCADE,
 new_request_id uuid NOT NULL UNIQUE REFERENCES pangu.submissions(id) ON DELETE CASCADE,
 created_at timestamptz NOT NULL DEFAULT now()
);

CREATE OR REPLACE FUNCTION pangu.purge_approval_items() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
 IF NEW.subject IS NULL THEN
  NEW.items='[]'::jsonb;
  NEW.invalidation_reason='';
  UPDATE pangu.approval_rebuild_intents SET reason=''
   WHERE run_id=NEW.run_id AND node_id=NEW.node_id AND from_generation=NEW.approval_generation;
 END IF;
 RETURN NEW;
END $$;
