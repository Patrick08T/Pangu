package scheduler

// UNION (not UNION ALL) bounds cycles. Scope filtering occurs before traversal;
// immutable ancestry remains available after private bodies have been erased.
const evolutionConsumerVersionsSQL = `WITH RECURSIVE visible AS (
 SELECT v.id,v.public_baseline,v.project_previous,
 CASE WHEN v.source_kind='auto' THEN b.spec ELSE v.spec END AS spec
 FROM pangu.catalog_versions v JOIN pangu.catalog_objects o ON o.id=v.object_id
 LEFT JOIN pangu.catalog_version_bodies b ON b.version_id=v.id
 WHERE o.namespace=$1 AND (v.project_id IS NULL OR v.project_id=$2)),
 edges AS (
 SELECT id,public_baseline AS parent FROM visible WHERE public_baseline IS NOT NULL
 UNION SELECT id,project_previous FROM visible WHERE project_previous IS NOT NULL
 UNION SELECT v.id,(n->>'agent_version')::uuid FROM visible v
 CROSS JOIN LATERAL jsonb_array_elements(coalesce(nullif(v.spec#>'{flow,nodes}','null'::jsonb),'[]'::jsonb)) n
 WHERE nullif(n->>'agent_version','') IS NOT NULL),
 affected(id) AS (
 SELECT id FROM visible WHERE id=ANY($3::uuid[])
 UNION SELECT e.id FROM edges e JOIN affected a ON a.id=e.parent)
 SELECT id::text FROM affected ORDER BY id`

// The intent generation is fenced, but every execution generation is retained:
// a prior generation without a matching independent stop proof remains unknown.
const evolutionConsumersSQL = `SELECT s.id::text,coalesce(s.run_id::text,''),
 e.request_id IS NOT NULL,coalesce(p.actor_id::text,'')
 FROM pangu.submissions s LEFT JOIN pangu.evolution_executions e ON e.request_id=s.id
 LEFT JOIN pangu.evolution_intents i ON i.id=e.intent_id AND i.namespace=s.namespace AND i.project_id=s.project_id
 LEFT JOIN pangu.experience_policies p ON p.namespace=i.namespace AND p.project_id=i.project_id AND p.revision=i.policy_revision
 WHERE s.namespace=$1 AND s.project_id=$2 AND (s.snapshot#>>'{workflow,version_id}'=ANY($3::text[])
 OR EXISTS(SELECT 1 FROM jsonb_array_elements(coalesce(nullif(s.snapshot#>'{workflow,nodes}','null'::jsonb),'[]'::jsonb)) n WHERE n->>'agent_version'=ANY($3::text[])))
 UNION
 SELECT e.request_id::text,coalesce(s.run_id::text,''),true,coalesce(p.actor_id::text,'')
 FROM pangu.evolution_executions e JOIN pangu.evolution_intents i ON i.id=e.intent_id
 LEFT JOIN pangu.submissions s ON s.id=e.request_id AND s.namespace=i.namespace AND s.project_id=i.project_id
 LEFT JOIN pangu.experience_policies p ON p.namespace=i.namespace AND p.project_id=i.project_id AND p.revision=i.policy_revision
 WHERE i.namespace=$1 AND i.project_id=$2 AND i.id=$4 AND i.generation=$5
 ORDER BY 1`

const evolutionConsumersUnreferencedSQL = `SELECT NOT (
 EXISTS(SELECT 1 FROM pangu.evolution_bindings b WHERE b.namespace=$1 AND b.project_id=$2
 AND (b.adopted_version=ANY($3::uuid[]) OR b.stable_version=ANY($3::uuid[])))
 OR EXISTS(SELECT 1 FROM pangu.schedules s WHERE s.namespace=$1 AND s.project_id=$2 AND NOT s.deleted
 AND (s.workflow->>'version_id'=ANY($3::text[]) OR EXISTS(
 SELECT 1 FROM jsonb_array_elements(coalesce(nullif(s.workflow->'nodes','null'::jsonb),'[]'::jsonb)) n WHERE n->>'agent_version'=ANY($3::text[]))))
 OR EXISTS(SELECT 1 FROM pangu.github_hooks h WHERE h.namespace=$1 AND h.project_id=$2 AND h.version_id=ANY($3::uuid[]))
 OR EXISTS(SELECT 1 FROM pangu.catalog_versions v JOIN pangu.catalog_objects o ON o.id=v.object_id
 LEFT JOIN pangu.catalog_version_bodies b ON b.version_id=v.id
 WHERE o.namespace=$1 AND (v.project_id IS NULL OR v.project_id=$2) AND v.id=ANY($3::uuid[])
 AND NOT v.id=ANY($4::uuid[]) AND (v.source_kind='manual' OR b.spec IS NOT NULL)))`
