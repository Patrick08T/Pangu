CREATE OR REPLACE FUNCTION pangu.purge_run_payloads(target uuid) RETURNS void LANGUAGE sql AS $$
 UPDATE pangu.submissions s SET snapshot=jsonb_strip_nulls(jsonb_build_object(
  'workflow',jsonb_build_object('project_id',s.project_id,'version_id',s.snapshot#>'{workflow,version_id}','budget',s.snapshot#>'{workflow,budget}'),
  'fork',CASE WHEN s.snapshot ? 'fork' THEN (s.snapshot->'fork')-'reused' ELSE NULL END,
  'created_by',s.snapshot->'created_by','source_run_id',s.snapshot->'source_run_id')) WHERE run_id=target;
 UPDATE pangu.schedule_fires f SET snapshot=s.snapshot FROM pangu.submissions s WHERE f.request_id=s.id AND s.run_id=target;
 UPDATE pangu.feedback SET reason='',payload_expired=true WHERE run_id=target;
 UPDATE pangu.approvals SET subject=NULL,reason='' WHERE run_id=target;
$$;
