-- Additive replacement: preserve trusted routing identity, never private evaluator bodies.
-- The execution session and retention transaction use the same advisory-lock key.
CREATE OR REPLACE FUNCTION pangu.purge_run_payloads(target uuid) RETURNS void LANGUAGE plpgsql AS $$
DECLARE request uuid;
BEGIN
 FOR request IN
  SELECT p.request_id FROM pangu.evolution_case_plans p
  JOIN pangu.evolution_executions e ON e.request_id=p.request_id
  LEFT JOIN pangu.evolution_case_runtime r ON r.request_id=p.request_id
  WHERE e.run_id=target OR r.run_id=target OR EXISTS(
   SELECT 1 FROM pangu.submissions s WHERE s.id=p.request_id AND s.run_id=target)
  ORDER BY p.request_id
 LOOP
  PERFORM pg_advisory_xact_lock(hashtextextended('evolution-case:'||request::text,0));
  IF NOT EXISTS(SELECT 1 FROM pangu.evolution_case_runtime r WHERE r.request_id=request
   AND r.run_id=target AND r.stop_confirmed AND r.status<>'running' AND r.finished_at IS NOT NULL)
   OR EXISTS(SELECT 1 FROM pangu.node_attempts a WHERE a.finished_at IS NULL AND
    (a.run_id=target OR a.node_run_id IN (SELECT n.node_run_id FROM pangu.evolution_node_runs n WHERE n.request_id=request))) THEN
   RAISE EXCEPTION 'retention cannot confirm stopped evolution case' USING ERRCODE='55000';
  END IF;
  UPDATE pangu.evolution_case_plans SET plan=NULL,erased_at=coalesce(erased_at,clock_timestamp()) WHERE request_id=request;
  UPDATE pangu.evolution_case_runtime SET outputs=NULL WHERE request_id=request;
  UPDATE pangu.evolution_node_runs SET output=NULL WHERE request_id=request;
 END LOOP;
 PERFORM pg_advisory_xact_lock(hashtextextended('trace:'||target::text,0));
 UPDATE pangu.submissions s SET snapshot=jsonb_strip_nulls(jsonb_build_object(
  'workflow',jsonb_build_object('project_id',s.project_id,'version_id',s.snapshot#>'{workflow,version_id}','budget',s.snapshot#>'{workflow,budget}'),
  'fork',CASE WHEN s.snapshot ? 'fork' THEN (s.snapshot->'fork')-'reused' ELSE NULL END,
  'purpose',s.snapshot->'purpose','entry_key',s.snapshot->'entry_key','entry_revision',s.snapshot->'entry_revision',
  'created_by',s.snapshot->'created_by','source_run_id',s.snapshot->'source_run_id')) WHERE run_id=target;
 UPDATE pangu.schedule_fires f SET snapshot=s.snapshot FROM pangu.submissions s WHERE f.request_id=s.id AND s.run_id=target;
 UPDATE pangu.feedback SET reason='',payload_expired=true WHERE run_id=target;
 UPDATE pangu.approvals SET subject=NULL,reason='' WHERE run_id=target;
 DELETE FROM pangu.trace_events WHERE run_id=target;
 INSERT INTO pangu.run_debug(run_id,expired) VALUES(target,true)
  ON CONFLICT(run_id) DO UPDATE SET expired=true,paused=false,step_requested=false,step_node='';
END $$;

-- A retry may neither restore outputs nor turn a tombstoned evaluator back into a runner.
CREATE FUNCTION pangu.guard_erased_evolution_output() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
 PERFORM pg_advisory_xact_lock(hashtextextended('evolution-case:'||NEW.request_id::text,0));
 IF EXISTS(SELECT 1 FROM pangu.evolution_case_plans p WHERE p.request_id=NEW.request_id AND p.erased_at IS NOT NULL)
  AND (TG_OP='INSERT' OR NEW.status='running' OR
   coalesce(to_jsonb(NEW)->TG_ARGV[0],'null'::jsonb)<>'null'::jsonb) THEN
  RAISE EXCEPTION 'erased evolution outputs cannot be restored';
 END IF;
 RETURN NEW;
END $$;
CREATE TRIGGER evolution_runtime_erasure_guard BEFORE INSERT OR UPDATE ON pangu.evolution_case_runtime
 FOR EACH ROW EXECUTE FUNCTION pangu.guard_erased_evolution_output('outputs');
CREATE TRIGGER evolution_node_erasure_guard BEFORE INSERT OR UPDATE ON pangu.evolution_node_runs
 FOR EACH ROW EXECUTE FUNCTION pangu.guard_erased_evolution_output('output');

CREATE INDEX evolution_case_retention_keyset ON pangu.evolution_case_plans(namespace,created_at,request_id)
 WHERE erased_at IS NULL;
