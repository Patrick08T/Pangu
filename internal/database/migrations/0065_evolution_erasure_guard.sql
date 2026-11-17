-- Runtime execution owns a SESSION lease on another connection. Reacquiring that
-- advisory lease in an output trigger deadlocks the runner against itself.
-- Row locking instead serializes the guard with plan tombstoning; purge still
-- owns the evolution-case advisory lease before it erases any private body.
CREATE OR REPLACE FUNCTION pangu.guard_erased_evolution_output() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE erased boolean;
BEGIN
 IF TG_OP='UPDATE' AND (
  NEW.request_id IS DISTINCT FROM OLD.request_id OR
  to_jsonb(NEW)->'run_id' IS DISTINCT FROM to_jsonb(OLD)->'run_id' OR
  to_jsonb(NEW)->'node_key' IS DISTINCT FROM to_jsonb(OLD)->'node_key' OR
  to_jsonb(NEW)->'node_run_id' IS DISTINCT FROM to_jsonb(OLD)->'node_run_id' OR
  to_jsonb(NEW)->'input_hash' IS DISTINCT FROM to_jsonb(OLD)->'input_hash') THEN
  RAISE EXCEPTION 'evolution runtime identity is immutable';
 END IF;
 SELECT p.erased_at IS NOT NULL INTO erased FROM pangu.evolution_case_plans p
  WHERE p.request_id=NEW.request_id FOR SHARE;
 IF erased
  AND (TG_OP='INSERT' OR NEW.status='running' OR
   coalesce(to_jsonb(NEW)->TG_ARGV[0],'null'::jsonb)<>'null'::jsonb) THEN
  RAISE EXCEPTION 'erased evolution outputs cannot be restored';
 END IF;
 RETURN NEW;
END $$;
