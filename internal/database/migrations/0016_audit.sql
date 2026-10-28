CREATE TABLE pangu.audit_events (
    id uuid PRIMARY KEY,
    namespace text NOT NULL,
    actor_id uuid NOT NULL,
    token_id uuid,
    request_id uuid NOT NULL,
    action text NOT NULL CHECK(length(action) BETWEEN 1 AND 200),
    object_id uuid,
    project_id uuid,
    result_id uuid,
    result_version integer,
    outcome text NOT NULL DEFAULT 'pending' CHECK(outcome IN ('pending','succeeded','rejected','failed')),
    http_status integer CHECK(http_status BETWEEN 100 AND 599),
    created_at timestamptz NOT NULL DEFAULT now(),
    finished_at timestamptz
);
CREATE INDEX audit_events_namespace_time ON pangu.audit_events(namespace,created_at DESC,id DESC);
CREATE INDEX audit_events_project_time ON pangu.audit_events(project_id,created_at DESC,id DESC);
