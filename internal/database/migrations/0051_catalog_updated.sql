UPDATE pangu.catalog_objects AS object
SET updated_at = latest.published_at
FROM (
    SELECT object_id, max(created_at) AS published_at
    FROM pangu.catalog_versions
    GROUP BY object_id
) AS latest
WHERE object.id = latest.object_id
  AND object.updated_at < latest.published_at;
