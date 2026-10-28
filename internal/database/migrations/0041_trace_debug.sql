CREATE TABLE pangu.trace_events (
 id bigserial PRIMARY KEY,
 run_id uuid NOT NULL,
 node_id text NOT NULL,
 node_run_id uuid NOT NULL,
 attempt integer NOT NULL CHECK (attempt > 0),
 span_id text NOT NULL,
 parent_span_id text NOT NULL DEFAULT '',
 kind text NOT NULL CHECK (kind IN ('node','model','tool')),
 phase text NOT NULL CHECK (phase IN ('start','end')),
 name text NOT NULL,
 occurred_at timestamptz NOT NULL,
 duration_ms bigint NOT NULL DEFAULT 0 CHECK (duration_ms >= 0),
 status text NOT NULL CHECK (status IN ('running','succeeded','failed','cancelled')),
 input text NOT NULL DEFAULT '',
 output text NOT NULL DEFAULT '',
 error text NOT NULL DEFAULT '',
 truncated boolean NOT NULL DEFAULT false
);
CREATE INDEX trace_events_run ON pangu.trace_events(run_id,id DESC);
CREATE INDEX trace_events_node ON pangu.trace_events(run_id,node_id,id DESC);

CREATE TABLE pangu.run_debug (
 run_id uuid PRIMARY KEY,
 paused boolean NOT NULL DEFAULT false,
 step_requested boolean NOT NULL DEFAULT false,
 step_node text NOT NULL DEFAULT '',
 expired boolean NOT NULL DEFAULT false,
 revision integer NOT NULL DEFAULT 1,
 updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE FUNCTION pangu.purge_trace_debug() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
 IF NEW.status='purged' THEN
  PERFORM pg_advisory_xact_lock(hashtextextended('trace:'||NEW.run_id::text,0));
  DELETE FROM pangu.trace_events WHERE run_id=NEW.run_id;
  INSERT INTO pangu.run_debug(run_id,expired) VALUES(NEW.run_id,true)
   ON CONFLICT(run_id) DO UPDATE SET expired=true,paused=false,step_requested=false,step_node='';
 END IF;
 RETURN NEW;
END $$;
CREATE TRIGGER purge_trace_debug AFTER INSERT OR UPDATE ON pangu.retention_records
 FOR EACH ROW EXECUTE FUNCTION pangu.purge_trace_debug();

CREATE OR REPLACE FUNCTION pangu.purge_run_payloads(target uuid) RETURNS void LANGUAGE plpgsql AS $$
BEGIN
 PERFORM pg_advisory_xact_lock(hashtextextended('trace:'||target::text,0));
 UPDATE pangu.submissions s SET snapshot=jsonb_strip_nulls(jsonb_build_object(
  'workflow',jsonb_build_object('project_id',s.project_id,'version_id',s.snapshot#>'{workflow,version_id}','budget',s.snapshot#>'{workflow,budget}'),
  'fork',CASE WHEN s.snapshot ? 'fork' THEN (s.snapshot->'fork')-'reused' ELSE NULL END,
  'created_by',s.snapshot->'created_by','source_run_id',s.snapshot->'source_run_id')) WHERE run_id=target;
 UPDATE pangu.schedule_fires f SET snapshot=s.snapshot FROM pangu.submissions s WHERE f.request_id=s.id AND s.run_id=target;
 UPDATE pangu.feedback SET reason='',payload_expired=true WHERE run_id=target;
 UPDATE pangu.approvals SET subject=NULL,reason='' WHERE run_id=target;
 DELETE FROM pangu.trace_events WHERE run_id=target;
 INSERT INTO pangu.run_debug(run_id,expired) VALUES(target,true)
  ON CONFLICT(run_id) DO UPDATE SET expired=true,paused=false,step_requested=false,step_node='';
END $$;
