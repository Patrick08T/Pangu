-- Cross-task experience foundation. Standalone: depends only on pre-0056 pangu schema.
CREATE TABLE pangu.experience_policies (
 namespace text NOT NULL, project_id uuid NOT NULL REFERENCES pangu.projects(id),
 revision bigint NOT NULL CHECK (revision > 0), actor_id text NOT NULL,
 policy jsonb NOT NULL, created_at timestamptz NOT NULL DEFAULT now(),
 PRIMARY KEY(namespace,project_id,revision)
);
CREATE TABLE pangu.experience_versions (
 id uuid PRIMARY KEY, namespace text NOT NULL,
 project_id uuid NOT NULL REFERENCES pangu.projects(id), entry_id uuid NOT NULL,
 revision bigint NOT NULL CHECK (revision > 0), body_hash text NOT NULL CHECK (body_hash ~ '^[0-9a-f]{64}$'),
 sources jsonb NOT NULL CHECK (jsonb_typeof(sources)='array' AND jsonb_array_length(sources)>0),
 expires_at timestamptz NOT NULL, created_by text NOT NULL, created_at timestamptz NOT NULL DEFAULT now(),
 UNIQUE(namespace,project_id,entry_id,revision)
);
CREATE TABLE pangu.experience_bodies (
 version_id uuid PRIMARY KEY REFERENCES pangu.experience_versions(id), content jsonb NOT NULL
);
CREATE TABLE pangu.experience_states (
 version_id uuid PRIMARY KEY REFERENCES pangu.experience_versions(id),
 validity text NOT NULL CHECK (validity IN ('candidate','active','superseded','revoked','expired')),
 tombstoned boolean NOT NULL DEFAULT false, invalid_reason text NOT NULL DEFAULT '',
 updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE TABLE pangu.experience_deliveries (
 version_id uuid NOT NULL REFERENCES pangu.experience_versions(id), revision bigint NOT NULL CHECK (revision>0),
 mode text NOT NULL CHECK (mode IN ('shadow','canary','serving','paused')),
 policy jsonb NOT NULL, evidence_id text NOT NULL DEFAULT '', actor_id text NOT NULL,
 created_at timestamptz NOT NULL DEFAULT now(), PRIMARY KEY(version_id,revision)
);
CREATE TABLE pangu.experience_watermarks (
 namespace text NOT NULL, project_id uuid NOT NULL REFERENCES pangu.projects(id),
 run_id text NOT NULL, generation bigint NOT NULL CHECK (generation>0), snapshot jsonb NOT NULL,
 created_at timestamptz NOT NULL DEFAULT now(), PRIMARY KEY(namespace,project_id,run_id,generation)
);
CREATE TABLE pangu.experience_usage (
 namespace text NOT NULL, project_id uuid NOT NULL REFERENCES pangu.projects(id),
 identity_hash text NOT NULL CHECK (identity_hash ~ '^[0-9a-f]{64}$'), usage jsonb NOT NULL,
 created_at timestamptz NOT NULL DEFAULT now(), PRIMARY KEY(namespace,project_id,identity_hash)
);
CREATE TABLE pangu.experience_artifacts (
 namespace text NOT NULL, project_id uuid NOT NULL REFERENCES pangu.projects(id),
 id text NOT NULL, artifact jsonb NOT NULL, created_at timestamptz NOT NULL DEFAULT now(),
 PRIMARY KEY(namespace,project_id,id)
);
CREATE TABLE pangu.experience_learning_intents (
 id uuid PRIMARY KEY, namespace text NOT NULL, project_id uuid NOT NULL REFERENCES pangu.projects(id),
 source_run_id text NOT NULL, evidence_revision bigint NOT NULL CHECK (evidence_revision>0),
 policy_revision bigint NOT NULL CHECK (policy_revision>0), intent jsonb NOT NULL,
 created_at timestamptz NOT NULL DEFAULT now(), updated_at timestamptz NOT NULL DEFAULT now(),
 UNIQUE(namespace,project_id,source_run_id,evidence_revision,policy_revision)
);
CREATE TABLE pangu.experience_events (
 id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY, namespace text NOT NULL,
 project_id uuid NOT NULL REFERENCES pangu.projects(id), actor_id text NOT NULL,
 action text NOT NULL, subject_id text NOT NULL, detail jsonb NOT NULL,
 created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX experience_versions_scope ON pangu.experience_versions(namespace,project_id,entry_id);
CREATE FUNCTION pangu.experience_immutable() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN RAISE EXCEPTION 'experience immutable record'; END;
$$;
CREATE TRIGGER experience_policies_immutable BEFORE UPDATE OR DELETE ON pangu.experience_policies
 FOR EACH ROW EXECUTE FUNCTION pangu.experience_immutable();
CREATE TRIGGER experience_versions_immutable BEFORE UPDATE OR DELETE ON pangu.experience_versions
 FOR EACH ROW EXECUTE FUNCTION pangu.experience_immutable();
-- Bodies may be erased for source withdrawal/retention, but never rewritten in place.
CREATE TRIGGER experience_bodies_immutable BEFORE UPDATE ON pangu.experience_bodies
 FOR EACH ROW EXECUTE FUNCTION pangu.experience_immutable();
CREATE TRIGGER experience_deliveries_immutable BEFORE UPDATE OR DELETE ON pangu.experience_deliveries
 FOR EACH ROW EXECUTE FUNCTION pangu.experience_immutable();
CREATE TRIGGER experience_watermarks_immutable BEFORE UPDATE OR DELETE ON pangu.experience_watermarks
 FOR EACH ROW EXECUTE FUNCTION pangu.experience_immutable();
CREATE TRIGGER experience_usage_immutable BEFORE UPDATE OR DELETE ON pangu.experience_usage
 FOR EACH ROW EXECUTE FUNCTION pangu.experience_immutable();
CREATE TRIGGER experience_artifacts_immutable BEFORE UPDATE OR DELETE ON pangu.experience_artifacts
 FOR EACH ROW EXECUTE FUNCTION pangu.experience_immutable();
CREATE TRIGGER experience_events_immutable BEFORE UPDATE OR DELETE ON pangu.experience_events
 FOR EACH ROW EXECUTE FUNCTION pangu.experience_immutable();
