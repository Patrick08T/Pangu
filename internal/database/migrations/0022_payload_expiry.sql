ALTER TABLE pangu.submissions ADD COLUMN input_hash text NOT NULL DEFAULT '';
UPDATE pangu.submissions SET input_hash=encode(sha256(convert_to(coalesce(snapshot#>>'{workflow,input}',''),'UTF8')),'hex');

UPDATE pangu.submissions s SET snapshot=jsonb_strip_nulls(jsonb_build_object(
  'workflow',jsonb_build_object('project_id',s.project_id,'version_id',s.snapshot#>'{workflow,version_id}'),
  'created_by',s.snapshot->'created_by','source_run_id',s.snapshot->'source_run_id'))
WHERE s.purged_at IS NOT NULL OR EXISTS(SELECT 1 FROM pangu.workspace_runs r WHERE r.run_id=s.run_id AND r.purged_at IS NOT NULL);
UPDATE pangu.schedule_fires f SET snapshot=s.snapshot FROM pangu.submissions s
WHERE f.request_id=s.id AND (s.purged_at IS NOT NULL OR EXISTS(SELECT 1 FROM pangu.workspace_runs r WHERE r.run_id=s.run_id AND r.purged_at IS NOT NULL));
