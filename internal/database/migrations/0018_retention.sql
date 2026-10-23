ALTER TABLE pangu.workspace_runs ADD COLUMN expired_at timestamptz;
ALTER TABLE pangu.workspace_runs ADD COLUMN purged_at timestamptz;
ALTER TABLE pangu.workspace_runs ADD COLUMN retention_checked_at timestamptz NOT NULL DEFAULT '-infinity';
CREATE INDEX workspace_runs_retention ON pangu.workspace_runs(retention_checked_at,created_at) WHERE purged_at IS NULL;
