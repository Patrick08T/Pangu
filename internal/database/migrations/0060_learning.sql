-- Learning trust bridge: immutable identities and erasable checker payloads.
CREATE TABLE pangu.learning_source_consents (
 id uuid PRIMARY KEY, namespace text NOT NULL, project_id uuid NOT NULL REFERENCES pangu.projects(id),
 source_id uuid NOT NULL, source_kind text NOT NULL CHECK(source_kind IN ('report','feedback')),
 run_id uuid NOT NULL, node_id text NOT NULL, source_hash text NOT NULL CHECK(source_hash ~ '^[0-9a-f]{64}$'),
 actor_id uuid NOT NULL REFERENCES pangu.users(id), expires_at timestamptz NOT NULL,
 created_at timestamptz NOT NULL DEFAULT now(), revoked_at timestamptz,
 UNIQUE(namespace,project_id,source_kind,source_id,source_hash)
);
CREATE TABLE pangu.learning_checkers (
 id uuid PRIMARY KEY, namespace text NOT NULL, project_id uuid NOT NULL REFERENCES pangu.projects(id),
 content_hash text NOT NULL CHECK(content_hash ~ '^[0-9a-f]{64}$'),
 config_hash text NOT NULL CHECK(config_hash ~ '^[0-9a-f]{64}$'),
 actor_id uuid NOT NULL REFERENCES pangu.users(id), expires_at timestamptz NOT NULL,
 created_at timestamptz NOT NULL DEFAULT now()
);
CREATE TABLE pangu.learning_checker_bodies (
 checker_id uuid PRIMARY KEY REFERENCES pangu.learning_checkers(id), body jsonb NOT NULL
);
CREATE TABLE pangu.learning_claim_requests (
 id uuid PRIMARY KEY, namespace text NOT NULL, project_id uuid NOT NULL REFERENCES pangu.projects(id),
 version_id uuid NOT NULL REFERENCES pangu.experience_versions(id),
 intent_id uuid NOT NULL REFERENCES pangu.experience_learning_intents(id), generation bigint NOT NULL CHECK(generation>0),
 checker_id uuid NOT NULL REFERENCES pangu.learning_checkers(id),
 source_set_hash text NOT NULL CHECK(source_set_hash ~ '^[0-9a-f]{64}$'),
 created_at timestamptz NOT NULL DEFAULT now(), UNIQUE(version_id,intent_id,generation,checker_id)
);
CREATE TABLE pangu.learning_claim_receipts (
 id uuid PRIMARY KEY REFERENCES pangu.learning_claim_requests(id),
 receipt_hash text NOT NULL CHECK(receipt_hash ~ '^[0-9a-f]{64}$'),
 checker_hash text NOT NULL CHECK(checker_hash ~ '^[0-9a-f]{64}$'),
 source_set_hash text NOT NULL CHECK(source_set_hash ~ '^[0-9a-f]{64}$'),
 passed boolean NOT NULL CHECK(passed), created_at timestamptz NOT NULL DEFAULT now()
);
CREATE TABLE pangu.learning_memory_observations (
 id uuid PRIMARY KEY, namespace text NOT NULL, project_id uuid NOT NULL REFERENCES pangu.projects(id),
 version_id uuid NOT NULL REFERENCES pangu.experience_versions(id), delivery_revision bigint NOT NULL CHECK(delivery_revision>0),
 policy_revision bigint NOT NULL CHECK(policy_revision>0), run_id uuid NOT NULL, task_id uuid NOT NULL,
 usage_hash text NOT NULL CHECK(usage_hash ~ '^[0-9a-f]{64}$'),
 source jsonb NOT NULL, accepted boolean NOT NULL, finished_at timestamptz NOT NULL,
 created_at timestamptz NOT NULL DEFAULT now(), UNIQUE(version_id,delivery_revision,task_id)
);
CREATE TABLE pangu.learning_events (
 id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY, namespace text NOT NULL,
 project_id uuid NOT NULL REFERENCES pangu.projects(id), actor_id uuid NOT NULL REFERENCES pangu.users(id),
 action text NOT NULL, subject_id text NOT NULL, detail jsonb NOT NULL, created_at timestamptz NOT NULL DEFAULT now()
);
CREATE FUNCTION pangu.learning_immutable() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN RAISE EXCEPTION 'learning immutable record'; END;
$$;
CREATE TRIGGER learning_checkers_immutable BEFORE UPDATE OR DELETE ON pangu.learning_checkers
 FOR EACH ROW EXECUTE FUNCTION pangu.learning_immutable();
CREATE TRIGGER learning_checker_bodies_immutable BEFORE UPDATE ON pangu.learning_checker_bodies
 FOR EACH ROW EXECUTE FUNCTION pangu.learning_immutable();
CREATE TRIGGER learning_claim_requests_immutable BEFORE UPDATE OR DELETE ON pangu.learning_claim_requests
 FOR EACH ROW EXECUTE FUNCTION pangu.learning_immutable();
CREATE TRIGGER learning_claim_receipts_immutable BEFORE UPDATE OR DELETE ON pangu.learning_claim_receipts
 FOR EACH ROW EXECUTE FUNCTION pangu.learning_immutable();
CREATE TRIGGER learning_memory_observations_immutable BEFORE UPDATE OR DELETE ON pangu.learning_memory_observations
 FOR EACH ROW EXECUTE FUNCTION pangu.learning_immutable();
CREATE TRIGGER learning_events_immutable BEFORE UPDATE OR DELETE ON pangu.learning_events
 FOR EACH ROW EXECUTE FUNCTION pangu.learning_immutable();
CREATE FUNCTION pangu.learning_consent_guard() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
 IF TG_OP='DELETE' THEN RAISE EXCEPTION 'learning consent tombstone is permanent'; END IF;
 IF (to_jsonb(NEW)-'revoked_at') IS DISTINCT FROM (to_jsonb(OLD)-'revoked_at') OR
    (OLD.revoked_at IS NOT NULL AND NEW.revoked_at IS DISTINCT FROM OLD.revoked_at) THEN
  RAISE EXCEPTION 'learning consent can only be withdrawn';
 END IF;
 RETURN NEW;
END $$;
CREATE TRIGGER learning_consent_guard BEFORE UPDATE OR DELETE ON pangu.learning_source_consents
 FOR EACH ROW EXECUTE FUNCTION pangu.learning_consent_guard();
