CREATE TABLE pangu.github_hooks (
 id uuid PRIMARY KEY,
 namespace text NOT NULL,
 project_id uuid NOT NULL REFERENCES pangu.projects(id),
 actor_id uuid NOT NULL REFERENCES pangu.users(id),
 version_id uuid NOT NULL REFERENCES pangu.catalog_versions(id),
 repository text NOT NULL,
 label text NOT NULL,
 secret_ref text NOT NULL,
 enabled boolean NOT NULL DEFAULT true,
 created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX github_hooks_project ON pangu.github_hooks(namespace,project_id,created_at DESC,id DESC);
CREATE TABLE pangu.github_events (
 hook_id uuid NOT NULL REFERENCES pangu.github_hooks(id),
 delivery_id uuid NOT NULL,
 payload_hash text NOT NULL,
 request_id uuid NOT NULL REFERENCES pangu.submissions(id) ON DELETE CASCADE,
 created_at timestamptz NOT NULL DEFAULT now(),
 PRIMARY KEY(hook_id,delivery_id)
);
