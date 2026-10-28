CREATE TABLE pangu.deliveries (
 id uuid PRIMARY KEY,
 namespace text NOT NULL,
 project_id uuid NOT NULL REFERENCES pangu.projects(id),
 actor_id uuid NOT NULL REFERENCES pangu.users(id),
 source_run_id uuid NOT NULL,
 report_id uuid NOT NULL,
 report_hash text NOT NULL,
 request_key text NOT NULL,
 payload_hash text NOT NULL,
 subject_hash text NOT NULL,
 definition jsonb,
 request_id uuid REFERENCES pangu.submissions(id) ON DELETE SET NULL,
 created_at timestamptz NOT NULL DEFAULT now(),
 expires_at timestamptz NOT NULL DEFAULT now()+interval '30 days',
 UNIQUE(namespace,request_key)
);
CREATE INDEX deliveries_project ON pangu.deliveries(namespace,project_id,created_at DESC,id DESC);
