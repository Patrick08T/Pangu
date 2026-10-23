ALTER TABLE pangu.workspace_runs ADD COLUMN finished_at timestamptz;
UPDATE pangu.workspace_runs SET finished_at=expired_at-interval '30 days' WHERE expired_at IS NOT NULL;
CREATE TABLE pangu.retention_records (
    id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    namespace text NOT NULL,
    project_id uuid NOT NULL,
    run_id uuid NOT NULL,
    worker_id text NOT NULL,
    status text NOT NULL CHECK(status IN ('purged','protected','leased','failed')),
    checked_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX retention_records_namespace_time ON pangu.retention_records(namespace,checked_at DESC,id DESC);
