ALTER TABLE pangu.schedules ADD COLUMN remote_synced boolean NOT NULL DEFAULT false;
ALTER TABLE pangu.schedules ADD COLUMN remote_checked_at timestamptz NOT NULL DEFAULT '-infinity';
CREATE INDEX schedules_remote_pending ON pangu.schedules(namespace,remote_checked_at,created_at) WHERE NOT remote_synced;
