-- 合并完全相同的配置副本，保留不可变的历史版本。
ALTER TABLE pangu.catalog_objects ADD COLUMN merged_into uuid REFERENCES pangu.catalog_objects(id);
CREATE TEMP TABLE catalog_duplicate_map ON COMMIT DROP AS
WITH signatures AS (
    SELECT o.id,o.namespace,o.kind,o.name,o.created_at,o.spec-'template_version' AS draft,
        (SELECT jsonb_agg(DISTINCT v.spec-'template_version') FROM pangu.catalog_versions v WHERE v.object_id=o.id) AS versions,
        (SELECT v.spec-'template_version' FROM pangu.catalog_versions v WHERE v.object_id=o.id ORDER BY number DESC LIMIT 1) AS latest
    FROM pangu.catalog_objects o WHERE NOT o.archived AND o.kind IN ('agent','workflow')
), ranked AS (
    SELECT id,first_value(id) OVER (PARTITION BY namespace,kind,name,draft,versions,latest ORDER BY created_at,id) AS canonical
    FROM signatures
)
SELECT id,canonical FROM ranked WHERE id<>canonical;

CREATE TEMP TABLE catalog_version_map ON COMMIT DROP AS
SELECT DISTINCT ON (old.id) old.id,kept.id AS canonical
FROM pangu.catalog_versions old
JOIN catalog_duplicate_map objects ON objects.id=old.object_id
JOIN pangu.catalog_versions kept ON kept.object_id=objects.canonical AND kept.spec-'template_version'=old.spec-'template_version'
ORDER BY old.id,kept.number;

WITH rewritten AS (
    SELECT o.id,jsonb_agg(CASE WHEN versions.canonical IS NULL THEN step.value
        ELSE jsonb_set(step.value,'{agent_version}',to_jsonb(versions.canonical::text)) END ORDER BY step.ordinality) AS nodes
    FROM pangu.catalog_objects o
    CROSS JOIN LATERAL jsonb_array_elements(o.spec#>'{flow,nodes}') WITH ORDINALITY AS step
    LEFT JOIN catalog_version_map versions ON versions.id::text=step.value->>'agent_version'
    WHERE o.kind='workflow'
    GROUP BY o.id
)
UPDATE pangu.catalog_objects o SET spec=jsonb_set(o.spec,'{flow,nodes}',r.nodes),revision=revision+1,updated_at=now()
FROM rewritten r WHERE o.id=r.id AND o.spec#>'{flow,nodes}' IS DISTINCT FROM r.nodes;

UPDATE pangu.catalog_objects o SET archived=true,merged_into=duplicates.canonical,updated_at=now()
FROM catalog_duplicate_map duplicates WHERE o.id=duplicates.id;
