ALTER TABLE pangu.projects ADD COLUMN vcs text NOT NULL DEFAULT 'git' CHECK (vcs IN ('git','perforce'));
ALTER TABLE pangu.projects ALTER COLUMN vcs DROP DEFAULT;
ALTER TABLE pangu.projects ADD COLUMN perforce jsonb;
CREATE TABLE pangu.perforce_sources (
    run_id uuid PRIMARY KEY,
    project_id uuid NOT NULL REFERENCES pangu.projects(id),
    config jsonb NOT NULL,
    template jsonb NOT NULL,
    baseline bigint NOT NULL CHECK (baseline > 0),
    client text NOT NULL UNIQUE,
    commit_id text NOT NULL DEFAULT '',
    created_at timestamptz NOT NULL DEFAULT now()
);
CREATE TABLE pangu.perforce_deliveries (
    run_id uuid PRIMARY KEY REFERENCES pangu.workspace_runs(run_id),
    checkpoint text NOT NULL,
    client text NOT NULL UNIQUE,
    changelist bigint,
    status text NOT NULL CHECK (status IN ('preparing','shelved','failed','empty')),
    error text NOT NULL DEFAULT '',
    updated_at timestamptz NOT NULL DEFAULT now()
);
