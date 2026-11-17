CREATE TABLE pangu.evolution_environments (
 namespace text NOT NULL, project_id uuid NOT NULL, id uuid NOT NULL,
 revision bigint NOT NULL CHECK(revision>0), spec jsonb NOT NULL, spec_hash text NOT NULL,
 created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 PRIMARY KEY(namespace,project_id,id,revision)
);
CREATE TABLE pangu.evolution_intents (
 id uuid PRIMARY KEY, namespace text NOT NULL, project_id uuid NOT NULL,
 policy_revision bigint NOT NULL CHECK(policy_revision>0), generation integer NOT NULL DEFAULT 1 CHECK(generation>0),
 state text NOT NULL CHECK(state IN ('preparing','evaluating','passed','publishing','switching','observing','stabilizing','rolling_back','reconciling','stable','rolled_back','rejected','invalidated','insufficient')),
 request_hash text NOT NULL, request jsonb NOT NULL, targets jsonb NOT NULL,
 experiment_id uuid REFERENCES pangu.evaluation_experiments(id), experiment_hash text,
 publication jsonb, created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 updated_at timestamptz NOT NULL DEFAULT clock_timestamp(), observing_at timestamptz,
 UNIQUE(namespace,project_id,id)
);
CREATE TABLE pangu.evolution_target_claims (
 namespace text NOT NULL, project_id uuid NOT NULL, target text NOT NULL,
 intent_id uuid NOT NULL REFERENCES pangu.evolution_intents(id), generation integer NOT NULL CHECK(generation>0),
 created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 PRIMARY KEY(namespace,project_id,target)
);
CREATE INDEX evolution_claim_owner ON pangu.evolution_target_claims(intent_id,generation);
CREATE TABLE pangu.evolution_budget_charges (
 namespace text NOT NULL, project_id uuid NOT NULL, kind text NOT NULL CHECK(kind IN ('learning','evaluation')),
 reservation_key text NOT NULL, policy_revision bigint NOT NULL CHECK(policy_revision>0),
 calls bigint NOT NULL CHECK(calls>0), tokens bigint NOT NULL CHECK(tokens>0),
 created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 PRIMARY KEY(namespace,project_id,kind,reservation_key)
);
CREATE INDEX evolution_budget_window ON pangu.evolution_budget_charges(namespace,project_id,kind,created_at);
CREATE TABLE pangu.evolution_bindings (
 namespace text NOT NULL, project_id uuid NOT NULL, entry_key text NOT NULL,
 revision bigint NOT NULL DEFAULT 1 CHECK(revision>0), adopted_version uuid NOT NULL REFERENCES pangu.catalog_versions(id),
 stable_version uuid NOT NULL REFERENCES pangu.catalog_versions(id), intent_id uuid REFERENCES pangu.evolution_intents(id),
 generation integer NOT NULL DEFAULT 0 CHECK(generation>=0),
 PRIMARY KEY(namespace,project_id,entry_key), CHECK((intent_id IS NULL)=(generation=0))
);
CREATE TABLE pangu.evolution_switches (
 intent_id uuid NOT NULL REFERENCES pangu.evolution_intents(id), generation integer NOT NULL,
 entry_key text NOT NULL, direction text NOT NULL CHECK(direction IN ('adopt','rollback','stabilize')),
 expected jsonb NOT NULL, result jsonb, status text NOT NULL CHECK(status IN ('preparing','pending','confirmed','conflict','unknown')),
 created_at timestamptz NOT NULL DEFAULT clock_timestamp(), updated_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 PRIMARY KEY(intent_id,generation,entry_key,direction)
);
CREATE TABLE pangu.evolution_executions (
 intent_id uuid NOT NULL REFERENCES pangu.evolution_intents(id), generation integer NOT NULL,
 arm text NOT NULL CHECK(arm IN ('baseline','candidate')), case_id uuid NOT NULL, repeat integer NOT NULL CHECK(repeat>0),
 input_hash text NOT NULL, request_id uuid NOT NULL UNIQUE, run_id uuid, node_id text,
 status text NOT NULL CHECK(status IN ('reserved','running','completed','failed','timeout','infrastructure_error')),
 max_calls bigint NOT NULL CHECK(max_calls>0), max_tokens bigint NOT NULL CHECK(max_tokens>0),
 created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 PRIMARY KEY(intent_id,generation,arm,case_id,repeat)
);
CREATE TABLE pangu.evolution_machine_results (
 request_id uuid PRIMARY KEY REFERENCES pangu.evolution_executions(request_id),
 run_id uuid NOT NULL, node_id text NOT NULL, result_hash text NOT NULL, result jsonb NOT NULL,
 created_at timestamptz NOT NULL DEFAULT clock_timestamp()
);
CREATE TABLE pangu.evolution_observations (
 intent_id uuid NOT NULL REFERENCES pangu.evolution_intents(id), generation integer NOT NULL,
 entry_key text NOT NULL, run_id uuid NOT NULL, source_hash text NOT NULL,
 sample jsonb NOT NULL, created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 PRIMARY KEY(intent_id,generation,entry_key,run_id)
);
CREATE FUNCTION pangu.protect_evolution_ledger() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN RAISE EXCEPTION 'evolution ledger is immutable'; END $$;
CREATE TRIGGER evolution_environment_immutable BEFORE UPDATE OR DELETE ON pangu.evolution_environments FOR EACH ROW EXECUTE FUNCTION pangu.protect_evolution_ledger();
CREATE TRIGGER evolution_budget_immutable BEFORE UPDATE OR DELETE ON pangu.evolution_budget_charges FOR EACH ROW EXECUTE FUNCTION pangu.protect_evolution_ledger();
CREATE TRIGGER evolution_result_immutable BEFORE UPDATE OR DELETE ON pangu.evolution_machine_results FOR EACH ROW EXECUTE FUNCTION pangu.protect_evolution_ledger();
CREATE TRIGGER evolution_observation_immutable BEFORE UPDATE OR DELETE ON pangu.evolution_observations FOR EACH ROW EXECUTE FUNCTION pangu.protect_evolution_ledger();
