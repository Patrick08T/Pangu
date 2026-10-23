ALTER TABLE pangu.submissions ADD COLUMN finished_at timestamptz;
ALTER TABLE pangu.submissions ADD COLUMN purged_at timestamptz;
ALTER TABLE pangu.submissions ADD COLUMN retention_checked_at timestamptz NOT NULL DEFAULT '-infinity';
CREATE INDEX submissions_retention ON pangu.submissions(retention_checked_at,created_at) WHERE run_id IS NOT NULL AND purged_at IS NULL;
CREATE INDEX retention_records_run ON pangu.retention_records(run_id) WHERE status='purged';
