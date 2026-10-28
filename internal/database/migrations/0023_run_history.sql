ALTER TABLE pangu.submissions ADD COLUMN history_checked_at timestamptz NOT NULL DEFAULT '-infinity';
CREATE TABLE pangu.run_history (
    run_id uuid PRIMARY KEY REFERENCES pangu.submissions(run_id) ON DELETE CASCADE,
    namespace text NOT NULL,
    finished_at timestamptz NOT NULL,
    summary jsonb NOT NULL
);
CREATE INDEX submissions_history_pending ON pangu.submissions(namespace,history_checked_at) WHERE run_id IS NOT NULL;
CREATE INDEX submissions_run_list ON pangu.submissions(namespace,created_at DESC,run_id DESC) WHERE run_id IS NOT NULL;
