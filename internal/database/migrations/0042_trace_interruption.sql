CREATE INDEX trace_events_span ON pangu.trace_events(run_id,span_id,phase);

CREATE FUNCTION pangu.finish_interrupted_trace() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
 PERFORM pg_advisory_xact_lock(hashtextextended('trace:'||NEW.run_id::text,0));
 INSERT INTO pangu.trace_events(run_id,node_id,node_run_id,attempt,span_id,parent_span_id,kind,phase,name,occurred_at,duration_ms,status,error)
 SELECT e.run_id,e.node_id,e.node_run_id,e.attempt,e.span_id,e.parent_span_id,e.kind,'end',e.name,NEW.finished_at,
 greatest(0,extract(epoch FROM (NEW.finished_at-e.occurred_at))*1000)::bigint,
 CASE WHEN NEW.status='cancelled' THEN 'cancelled' ELSE 'failed' END,
 '执行器已停止，未收到该操作的完成回执；不代表操作成功或外部副作用已撤销'
 FROM pangu.trace_events e WHERE e.run_id=NEW.run_id AND e.node_run_id=NEW.node_run_id AND e.attempt=NEW.attempt AND e.phase='start'
 AND NOT EXISTS(SELECT 1 FROM pangu.trace_events done WHERE done.run_id=e.run_id AND done.span_id=e.span_id AND done.phase='end')
 AND NOT EXISTS(SELECT 1 FROM pangu.run_debug WHERE run_id=NEW.run_id AND expired);
 RETURN NEW;
END $$;
CREATE TRIGGER finish_interrupted_trace AFTER UPDATE OF finished_at ON pangu.node_attempts
 FOR EACH ROW WHEN (OLD.finished_at IS NULL AND NEW.finished_at IS NOT NULL)
 EXECUTE FUNCTION pangu.finish_interrupted_trace();
