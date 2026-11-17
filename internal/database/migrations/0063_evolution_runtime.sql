-- Private evaluator execution bodies are separate from immutable identity/digest metadata.
CREATE TABLE pangu.evolution_case_plans (
 request_id uuid PRIMARY KEY REFERENCES pangu.evolution_executions(request_id),
 namespace text NOT NULL, project_id uuid NOT NULL REFERENCES pangu.projects(id),
 actor_id uuid NOT NULL REFERENCES pangu.users(id), plan jsonb,
 plan_hash text NOT NULL CHECK(plan_hash ~ '^[0-9a-f]{64}$'),
 created_at timestamptz NOT NULL DEFAULT clock_timestamp(), erased_at timestamptz,
 CHECK((plan IS NULL)=(erased_at IS NOT NULL))
);
CREATE INDEX evolution_case_plan_retention ON pangu.evolution_case_plans(namespace,project_id,created_at);
CREATE TABLE pangu.evolution_case_runtime (
 request_id uuid PRIMARY KEY REFERENCES pangu.evolution_case_plans(request_id),
 run_id uuid UNIQUE NOT NULL,
 status text NOT NULL CHECK(status IN ('running','completed','failed','timeout','infrastructure_error')),
 outputs jsonb, started_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 finished_at timestamptz, stop_confirmed boolean NOT NULL DEFAULT false,
 CHECK(NOT stop_confirmed OR (status<>'running' AND finished_at IS NOT NULL))
);
CREATE TABLE pangu.evolution_node_runs (
 request_id uuid NOT NULL REFERENCES pangu.evolution_case_plans(request_id), node_key text NOT NULL,
 input_hash text NOT NULL CHECK(input_hash ~ '^[0-9a-f]{64}$'), node_run_id uuid UNIQUE NOT NULL,
 status text NOT NULL CHECK(status IN ('running','completed','failed')),
 output jsonb, started_at timestamptz NOT NULL DEFAULT clock_timestamp(), finished_at timestamptz,
 PRIMARY KEY(request_id,node_key)
);
CREATE TABLE pangu.evolution_remote_intents (
 effect_key text PRIMARY KEY, namespace text NOT NULL,
 project_id uuid NOT NULL REFERENCES pangu.projects(id), target_id uuid NOT NULL,
 revision bigint NOT NULL CHECK(revision>0), expected jsonb NOT NULL, result jsonb NOT NULL,
 status text NOT NULL CHECK(status IN ('pending','running','confirmed','conflict','unknown')),
 remote_id uuid, created_at timestamptz NOT NULL DEFAULT clock_timestamp(), finished_at timestamptz
);
CREATE INDEX evolution_remote_pending ON pangu.evolution_remote_intents(namespace,status,created_at);

CREATE FUNCTION pangu.protect_evolution_case_plan() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
 IF TG_OP='DELETE' THEN RAISE EXCEPTION 'evolution execution identity is immutable'; END IF;
 IF (to_jsonb(NEW)-'plan'-'erased_at') IS DISTINCT FROM (to_jsonb(OLD)-'plan'-'erased_at')
  OR (NEW.plan IS NOT NULL AND NEW.plan IS DISTINCT FROM OLD.plan)
  OR (OLD.erased_at IS NOT NULL AND NEW.erased_at IS DISTINCT FROM OLD.erased_at) THEN
  RAISE EXCEPTION 'evolution execution plan is immutable except body erasure';
 END IF;
 RETURN NEW;
END $$;
CREATE TRIGGER evolution_case_plan_immutable BEFORE UPDATE OR DELETE ON pangu.evolution_case_plans
 FOR EACH ROW EXECUTE FUNCTION pangu.protect_evolution_case_plan();

ALTER TABLE pangu.github_hooks ADD COLUMN revision bigint NOT NULL DEFAULT 1 CHECK(revision>0);
CREATE FUNCTION pangu.advance_github_hook_revision() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN NEW.revision=OLD.revision+1; RETURN NEW; END $$;
CREATE TRIGGER github_hook_revision BEFORE UPDATE ON pangu.github_hooks
 FOR EACH ROW EXECUTE FUNCTION pangu.advance_github_hook_revision();
