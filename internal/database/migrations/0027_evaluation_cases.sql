CREATE TABLE pangu.evaluation_cases (
 id uuid PRIMARY KEY,
 namespace text NOT NULL,
 project_id uuid NOT NULL REFERENCES pangu.projects(id),
 actor_id uuid NOT NULL REFERENCES pangu.users(id),
 name text NOT NULL,
 input text NOT NULL,
 input_hash text NOT NULL,
 expected text NOT NULL,
 criteria jsonb NOT NULL,
 archived boolean NOT NULL DEFAULT false,
 created_at timestamptz NOT NULL DEFAULT now(),
 expires_at timestamptz NOT NULL DEFAULT now()+interval '30 days'
);
CREATE INDEX evaluation_cases_project ON pangu.evaluation_cases(namespace,project_id,created_at DESC,id DESC);
CREATE TABLE pangu.evaluation_runs (
 case_id uuid NOT NULL REFERENCES pangu.evaluation_cases(id) ON DELETE CASCADE,
 version_id uuid NOT NULL REFERENCES pangu.catalog_versions(id),
 request_id uuid NOT NULL REFERENCES pangu.submissions(id) ON DELETE CASCADE,
 created_at timestamptz NOT NULL DEFAULT now(),
 PRIMARY KEY(case_id,request_id)
);
