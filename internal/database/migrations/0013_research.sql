CREATE TABLE pangu.research_reports (
    id uuid PRIMARY KEY,
    run_id uuid NOT NULL REFERENCES pangu.workspace_runs(run_id),
    node_run_id uuid NOT NULL UNIQUE,
    title text NOT NULL,
    body bytea NOT NULL CHECK (octet_length(body) <= 1048576),
    output jsonb NOT NULL,
    evidence jsonb NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX research_reports_run ON pangu.research_reports(run_id,created_at,id);
