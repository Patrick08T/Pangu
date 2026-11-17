CREATE TABLE pangu.plan_continuations (
    approval_id uuid PRIMARY KEY REFERENCES pangu.approvals(id),
    request_id uuid NOT NULL UNIQUE REFERENCES pangu.submissions(id),
    version_id uuid NOT NULL REFERENCES pangu.catalog_versions(id),
    created_at timestamptz NOT NULL DEFAULT now()
);
