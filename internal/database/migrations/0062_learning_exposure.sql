-- Permanent exposure identities survive consent withdrawal and source retention cleanup.
CREATE TABLE pangu.learning_source_exposures (
 consent_id uuid PRIMARY KEY REFERENCES pangu.learning_source_consents(id),
 namespace text NOT NULL, project_id uuid NOT NULL,
 source_id uuid NOT NULL, run_id uuid NOT NULL, task_id uuid,
 input_hash text CHECK(input_hash IS NULL OR input_hash ~ '^[0-9a-f]{64}$'),
 unresolved boolean NOT NULL, created_at timestamptz NOT NULL DEFAULT now(),
 CHECK(unresolved OR (task_id IS NOT NULL AND input_hash IS NOT NULL))
);
CREATE INDEX learning_exposure_project ON pangu.learning_source_exposures(namespace,project_id);
INSERT INTO pangu.learning_source_exposures(consent_id,namespace,project_id,source_id,run_id,task_id,input_hash,unresolved,created_at)
 SELECT c.id,c.namespace,c.project_id,c.source_id,c.run_id,s.task_id,
 CASE WHEN nullif(s.snapshot->'workflow'->>'input','') IS NOT NULL
      THEN encode(sha256(convert_to(s.snapshot->'workflow'->>'input','UTF8')),'hex') END,
 s.task_id IS NULL OR nullif(s.snapshot->'workflow'->>'input','') IS NULL,c.created_at
 FROM pangu.learning_source_consents c LEFT JOIN pangu.submissions s
 ON s.namespace=c.namespace AND s.project_id=c.project_id AND s.run_id=c.run_id;
CREATE TRIGGER learning_source_exposures_immutable BEFORE UPDATE OR DELETE ON pangu.learning_source_exposures
 FOR EACH ROW EXECUTE FUNCTION pangu.learning_immutable();
