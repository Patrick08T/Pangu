CREATE TABLE pangu.submissions (
    id uuid PRIMARY KEY,
    namespace text NOT NULL,
    project_id uuid NOT NULL REFERENCES pangu.projects(id),
    operation text NOT NULL,
    key_hash text NOT NULL,
    payload_hash text NOT NULL,
    snapshot jsonb NOT NULL,
    run_id uuid,
    created_at timestamptz NOT NULL DEFAULT now(),
    last_checked_at timestamptz NOT NULL DEFAULT '-infinity',
    last_error text NOT NULL DEFAULT '',
    UNIQUE(namespace,project_id,operation,key_hash),
    UNIQUE(run_id)
);
CREATE INDEX submissions_pending ON pangu.submissions(namespace,last_checked_at,created_at) WHERE run_id IS NULL;
