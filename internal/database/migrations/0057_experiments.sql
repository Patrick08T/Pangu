-- 受保护评测：不可变配置与可耗用的套件级暴露额度分离。
CREATE TABLE pangu.evaluation_holdout_pools (
 id uuid PRIMARY KEY,
 namespace text NOT NULL,
 project_id uuid NOT NULL REFERENCES pangu.projects(id),
 holdout_hash text NOT NULL CHECK (holdout_hash ~ '^[0-9a-f]{64}$'),
 rules_hash text NOT NULL CHECK (rules_hash ~ '^[0-9a-f]{64}$'),
 max_candidates integer NOT NULL CHECK (max_candidates > 0),
 used integer NOT NULL DEFAULT 0 CHECK (used >= 0 AND used <= max_candidates),
 UNIQUE(namespace,project_id,holdout_hash)
);

CREATE TABLE pangu.evaluation_suites (
 id uuid PRIMARY KEY,
 namespace text NOT NULL,
 project_id uuid NOT NULL REFERENCES pangu.projects(id),
 pool_id uuid NOT NULL REFERENCES pangu.evaluation_holdout_pools(id),
 spec jsonb NOT NULL,
 spec_hash text NOT NULL CHECK (spec_hash ~ '^[0-9a-f]{64}$'),
 created_at timestamptz NOT NULL DEFAULT now()
);

-- 不复制案例正文，不阻止保留期清理；来源缺失/撤回时服务端拒绝使用。
CREATE TABLE pangu.evaluation_suite_cases (
 suite_id uuid NOT NULL REFERENCES pangu.evaluation_suites(id),
 case_id uuid NOT NULL,
 partition text NOT NULL CHECK (partition IN ('development','protection','holdout')),
 input_hash text NOT NULL CHECK (input_hash ~ '^[0-9a-f]{64}$'),
 family_id text NOT NULL,
 task_id uuid NOT NULL,
 source_run_id uuid NOT NULL,
 PRIMARY KEY(suite_id,case_id)
);
CREATE INDEX evaluation_suite_cases_case ON pangu.evaluation_suite_cases(case_id);

-- 题目身份跨套件保留，不能改名/加题/换策略后获得新暴露预算。
CREATE TABLE pangu.evaluation_case_allocations (
 namespace text NOT NULL,
 project_id uuid NOT NULL REFERENCES pangu.projects(id),
 input_hash text NOT NULL CHECK (input_hash ~ '^[0-9a-f]{64}$'),
 partition text NOT NULL CHECK (partition IN ('development','protection','holdout')),
 pool_id uuid NOT NULL REFERENCES pangu.evaluation_holdout_pools(id),
 family_id text NOT NULL,
 task_id uuid NOT NULL,
 source_run_id uuid NOT NULL,
 PRIMARY KEY(namespace,project_id,input_hash)
);
CREATE TABLE pangu.evaluation_identity_allocations (
 namespace text NOT NULL,
 project_id uuid NOT NULL REFERENCES pangu.projects(id),
 identity_kind text NOT NULL CHECK (identity_kind IN ('family','task','source')),
 identity_key text NOT NULL,
 partition text NOT NULL CHECK (partition IN ('development','protection','holdout')),
 pool_id uuid NOT NULL REFERENCES pangu.evaluation_holdout_pools(id),
 PRIMARY KEY(namespace,project_id,identity_kind,identity_key)
);

CREATE TABLE pangu.evaluation_experiments (
 id uuid PRIMARY KEY,
 namespace text NOT NULL,
 project_id uuid NOT NULL REFERENCES pangu.projects(id),
 suite_id uuid NOT NULL REFERENCES pangu.evaluation_suites(id),
 generation integer NOT NULL CHECK (generation > 0),
 status text NOT NULL DEFAULT 'active' CHECK (status IN ('active','invalidated')),
 spec jsonb NOT NULL,
 spec_hash text NOT NULL CHECK (spec_hash ~ '^[0-9a-f]{64}$'),
 created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX evaluation_experiments_scope ON pangu.evaluation_experiments(namespace,project_id,id);

CREATE TABLE pangu.evaluation_experiment_receipts (
 experiment_id uuid NOT NULL REFERENCES pangu.evaluation_experiments(id),
 generation integer NOT NULL CHECK (generation > 0),
 arm text NOT NULL CHECK (arm IN ('baseline','candidate')),
 case_id uuid NOT NULL,
 repeat integer NOT NULL CHECK (repeat > 0),
 attempt integer NOT NULL CHECK (attempt = 1),
 receipt jsonb NOT NULL,
 receipt_hash text NOT NULL CHECK (receipt_hash ~ '^[0-9a-f]{64}$'),
 created_at timestamptz NOT NULL DEFAULT now(),
 PRIMARY KEY(experiment_id,generation,arm,case_id,repeat)
);

CREATE FUNCTION pangu.guard_evaluation_immutable() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
 RAISE EXCEPTION '受保护评测记录不可变';
END $$;
CREATE TRIGGER evaluation_suites_immutable BEFORE UPDATE OR DELETE ON pangu.evaluation_suites
 FOR EACH ROW EXECUTE FUNCTION pangu.guard_evaluation_immutable();
CREATE TRIGGER evaluation_suite_cases_immutable BEFORE UPDATE OR DELETE ON pangu.evaluation_suite_cases
 FOR EACH ROW EXECUTE FUNCTION pangu.guard_evaluation_immutable();
CREATE TRIGGER evaluation_allocations_immutable BEFORE UPDATE OR DELETE ON pangu.evaluation_case_allocations
 FOR EACH ROW EXECUTE FUNCTION pangu.guard_evaluation_immutable();
CREATE TRIGGER evaluation_identity_allocations_immutable BEFORE UPDATE OR DELETE ON pangu.evaluation_identity_allocations
 FOR EACH ROW EXECUTE FUNCTION pangu.guard_evaluation_immutable();
CREATE TRIGGER evaluation_receipts_immutable BEFORE UPDATE OR DELETE ON pangu.evaluation_experiment_receipts
 FOR EACH ROW EXECUTE FUNCTION pangu.guard_evaluation_immutable();

CREATE FUNCTION pangu.guard_evaluation_budget() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
 IF TG_OP='DELETE' THEN RAISE EXCEPTION '保留集额度不可删除'; END IF;
 IF (to_jsonb(NEW)-'used') IS DISTINCT FROM (to_jsonb(OLD)-'used') OR NEW.used < OLD.used THEN
  RAISE EXCEPTION '保留集额度不可重置';
 END IF;
 RETURN NEW;
END $$;
CREATE TRIGGER evaluation_holdout_budget_guard BEFORE UPDATE OR DELETE ON pangu.evaluation_holdout_pools
 FOR EACH ROW EXECUTE FUNCTION pangu.guard_evaluation_budget();

CREATE FUNCTION pangu.guard_evaluation_generation() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
 IF TG_OP='DELETE' THEN RAISE EXCEPTION '实验身份不可删除'; END IF;
 IF (to_jsonb(NEW)-'generation'-'status') IS DISTINCT FROM (to_jsonb(OLD)-'generation'-'status')
    OR OLD.status <> 'active' OR NEW.status <> 'invalidated' OR NEW.generation <> OLD.generation+1 THEN
  RAISE EXCEPTION '只允许使实验旧代际失效';
 END IF;
 RETURN NEW;
END $$;
CREATE TRIGGER evaluation_experiment_generation_guard BEFORE UPDATE OR DELETE ON pangu.evaluation_experiments
 FOR EACH ROW EXECUTE FUNCTION pangu.guard_evaluation_generation();

-- 仅可信服务从保存的回执重算后追加；发布读取同代际最新记录并核对哈希。
CREATE TABLE pangu.evaluation_experiment_decisions (
 id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
 experiment_id uuid NOT NULL REFERENCES pangu.evaluation_experiments(id),
 generation integer NOT NULL CHECK (generation > 0),
 experiment_hash text NOT NULL CHECK (experiment_hash ~ '^[0-9a-f]{64}$'),
 receipt_set_hash text NOT NULL CHECK (receipt_set_hash ~ '^[0-9a-f]{64}$'),
 decision_hash text NOT NULL CHECK (decision_hash ~ '^[0-9a-f]{64}$'),
 outcome text NOT NULL CHECK (outcome IN ('passed','rejected','insufficient')),
 category text NOT NULL,
 created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX evaluation_experiment_decisions_latest
 ON pangu.evaluation_experiment_decisions(experiment_id,generation,id DESC);
CREATE TRIGGER evaluation_decisions_immutable BEFORE UPDATE OR DELETE ON pangu.evaluation_experiment_decisions
 FOR EACH ROW EXECUTE FUNCTION pangu.guard_evaluation_immutable();
