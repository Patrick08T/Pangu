CREATE TABLE pangu.run_cancellations (
    run_id uuid PRIMARY KEY,
    namespace text NOT NULL,
    requested_at timestamptz NOT NULL DEFAULT now(),
    reconciled_at timestamptz
);
CREATE INDEX run_cancellations_pending ON pangu.run_cancellations(namespace, requested_at)
    WHERE reconciled_at IS NULL;

CREATE TABLE pangu.node_attempts (
    run_id uuid NOT NULL,
    node_run_id uuid NOT NULL,
    node_id text NOT NULL,
    attempt integer NOT NULL CHECK (attempt > 0),
    started_at timestamptz NOT NULL DEFAULT now(),
    finished_at timestamptz,
    status text NOT NULL DEFAULT 'running' CHECK (status IN ('running','succeeded','failed','cancelled')),
    message text NOT NULL DEFAULT '',
    truncated boolean NOT NULL DEFAULT false,
    PRIMARY KEY (node_run_id, attempt)
);
CREATE INDEX node_attempts_run ON pangu.node_attempts(run_id, started_at);
