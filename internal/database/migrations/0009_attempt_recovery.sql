ALTER TABLE pangu.node_attempts ADD COLUMN owner_host text NOT NULL DEFAULT '';
ALTER TABLE pangu.node_attempts ADD COLUMN owner_pid integer NOT NULL DEFAULT 0;
ALTER TABLE pangu.node_attempts ADD COLUMN owner_stamp text NOT NULL DEFAULT '';
ALTER TABLE pangu.node_attempts ADD COLUMN process_scope text NOT NULL DEFAULT '';
CREATE INDEX node_attempts_owner ON pangu.node_attempts(owner_host,owner_pid) WHERE finished_at IS NULL;

CREATE TABLE pangu.workspace_attempts (
    run_id uuid NOT NULL REFERENCES pangu.workspace_runs(run_id),
    node_run_id uuid NOT NULL,
    attempt integer NOT NULL,
    directory text NOT NULL,
    base_commit text NOT NULL,
    status text NOT NULL CHECK(status IN ('preparing','running','failed','published')),
    created_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY(node_run_id,attempt)
);
