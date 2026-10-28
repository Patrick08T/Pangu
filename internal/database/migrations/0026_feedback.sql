CREATE TABLE pangu.feedback (
 id uuid PRIMARY KEY,
 namespace text NOT NULL,
 project_id uuid NOT NULL REFERENCES pangu.projects(id),
 run_id uuid NOT NULL UNIQUE REFERENCES pangu.submissions(run_id) ON DELETE CASCADE,
 task_id uuid NOT NULL,
 actor_id uuid NOT NULL REFERENCES pangu.users(id),
 revision integer NOT NULL DEFAULT 1 CHECK(revision>0),
 subject_hash text NOT NULL CHECK(length(subject_hash)=64),
 decision text NOT NULL CHECK(decision IN ('accepted','needs_changes','rejected')),
 first_decision text NOT NULL CHECK(first_decision IN ('accepted','needs_changes','rejected')),
 reason text NOT NULL,
 payload_expired boolean NOT NULL DEFAULT false,
 created_at timestamptz NOT NULL DEFAULT now(),
 updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX feedback_project ON pangu.feedback(namespace,project_id,task_id);
CREATE OR REPLACE FUNCTION pangu.purge_run_payloads(target uuid) RETURNS void LANGUAGE sql AS $$
 UPDATE pangu.submissions s SET snapshot=jsonb_strip_nulls(jsonb_build_object(
  'workflow',jsonb_build_object('project_id',s.project_id,'version_id',s.snapshot#>'{workflow,version_id}'),
  'created_by',s.snapshot->'created_by','source_run_id',s.snapshot->'source_run_id')) WHERE run_id=target;
 UPDATE pangu.schedule_fires f SET snapshot=s.snapshot FROM pangu.submissions s WHERE f.request_id=s.id AND s.run_id=target;
 UPDATE pangu.feedback SET reason='',payload_expired=true WHERE run_id=target;
$$;
