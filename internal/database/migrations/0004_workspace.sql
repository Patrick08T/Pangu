CREATE TABLE pangu.projects (
    id uuid PRIMARY KEY,
    namespace text NOT NULL,
    name text NOT NULL,
    source text NOT NULL,
    branch text NOT NULL,
    worker_id text NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX projects_namespace ON pangu.projects(namespace, created_at, id);

CREATE TABLE pangu.workspace_runs (
    run_id uuid PRIMARY KEY,
    project_id uuid NOT NULL REFERENCES pangu.projects(id),
    base_commit text NOT NULL,
    head_commit text NOT NULL,
    worker_id text NOT NULL,
    directory text NOT NULL,
    ready boolean NOT NULL DEFAULT false,
    created_at timestamptz NOT NULL DEFAULT now()
);
CREATE TABLE pangu.workspace_checkpoints (
    node_run_id uuid PRIMARY KEY,
    run_id uuid NOT NULL REFERENCES pangu.workspace_runs(run_id),
    commit_id text NOT NULL,
    output jsonb NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now()
);
