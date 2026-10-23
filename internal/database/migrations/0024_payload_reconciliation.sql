CREATE FUNCTION pangu.purge_run_payloads(target uuid) RETURNS void LANGUAGE sql AS $$
 UPDATE pangu.submissions s SET snapshot=jsonb_strip_nulls(jsonb_build_object(
  'workflow',jsonb_build_object('project_id',s.project_id,'version_id',s.snapshot#>'{workflow,version_id}'),
  'created_by',s.snapshot->'created_by','source_run_id',s.snapshot->'source_run_id')) WHERE run_id=target;
 UPDATE pangu.schedule_fires f SET snapshot=s.snapshot FROM pangu.submissions s WHERE f.request_id=s.id AND s.run_id=target;
$$;
SELECT pangu.purge_run_payloads(run_id) FROM pangu.workspace_runs WHERE expired_at IS NOT NULL;
