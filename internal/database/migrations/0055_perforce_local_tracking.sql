ALTER TABLE pangu.workspace_attempts ADD COLUMN p4_tracked boolean NOT NULL DEFAULT false;
ALTER TABLE pangu.perforce_sources ADD COLUMN tracked_paths jsonb NOT NULL DEFAULT '[]';
