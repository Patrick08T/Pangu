ALTER TABLE pangu.perforce_sources DROP CONSTRAINT perforce_sources_client_key;
ALTER TABLE pangu.perforce_sources ADD COLUMN local_path text NOT NULL DEFAULT '';
ALTER TABLE pangu.perforce_sources ADD COLUMN work_change bigint;
ALTER TABLE pangu.perforce_sources ADD COLUMN local_commit text NOT NULL DEFAULT '';
ALTER TABLE pangu.perforce_sources ADD COLUMN existing_changes jsonb NOT NULL DEFAULT '[]';
ALTER TABLE pangu.workspace_attempts ADD COLUMN before_commit text NOT NULL DEFAULT '';
ALTER TABLE pangu.workspace_attempts ADD COLUMN after_commit text NOT NULL DEFAULT '';
