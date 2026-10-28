CREATE TABLE pangu.approvals (
 id uuid PRIMARY KEY,
 namespace text NOT NULL,
 project_id uuid NOT NULL REFERENCES pangu.projects(id),
 request_id uuid NOT NULL REFERENCES pangu.submissions(id) ON DELETE CASCADE,
 run_id uuid NOT NULL,
 node_id text NOT NULL,
 input_hash text NOT NULL,
 subject_hash text NOT NULL,
 subject jsonb,
 head_commit text NOT NULL,
 status text NOT NULL DEFAULT 'waiting' CHECK(status IN ('waiting','approved','rejected','revoked','consumed')),
 actor_id uuid REFERENCES pangu.users(id),
 reason text NOT NULL DEFAULT '',
 created_at timestamptz NOT NULL DEFAULT now(),
 expires_at timestamptz NOT NULL,
 decided_at timestamptz,
 consumed_at timestamptz,
 signalled_at timestamptz,
 UNIQUE(run_id,node_id)
);
CREATE INDEX approvals_namespace ON pangu.approvals(namespace,run_id);
CREATE INDEX approvals_outbox ON pangu.approvals(namespace) WHERE signalled_at IS NULL AND status IN ('approved','rejected','revoked');
CREATE OR REPLACE FUNCTION pangu.purge_run_payloads(target uuid) RETURNS void LANGUAGE sql AS $$
 UPDATE pangu.submissions s SET snapshot=jsonb_strip_nulls(jsonb_build_object(
  'workflow',jsonb_build_object('project_id',s.project_id,'version_id',s.snapshot#>'{workflow,version_id}'),
  'created_by',s.snapshot->'created_by','source_run_id',s.snapshot->'source_run_id')) WHERE run_id=target;
 UPDATE pangu.schedule_fires f SET snapshot=s.snapshot FROM pangu.submissions s WHERE f.request_id=s.id AND s.run_id=target;
 UPDATE pangu.feedback SET reason='',payload_expired=true WHERE run_id=target;
 UPDATE pangu.approvals SET subject=NULL,reason='' WHERE run_id=target;
$$;
