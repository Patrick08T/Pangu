ALTER TABLE pangu.node_attempts ADD COLUMN evidence jsonb NOT NULL DEFAULT '[]';
ALTER TABLE pangu.node_attempts ADD COLUMN evidence_truncated boolean NOT NULL DEFAULT false;
ALTER TABLE pangu.workspace_checkpoints ADD COLUMN evidence jsonb NOT NULL DEFAULT '[]';
ALTER TABLE pangu.workspace_checkpoints ADD COLUMN evidence_truncated boolean NOT NULL DEFAULT false;
