ALTER TABLE pangu.schedule_fires ADD COLUMN last_checked_at timestamptz NOT NULL DEFAULT '-infinity';
CREATE INDEX schedule_fires_pending ON pangu.schedule_fires(last_checked_at,created_at) WHERE status='pending';
