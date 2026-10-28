CREATE TABLE pangu.factory_sessions (
 id uuid PRIMARY KEY,
 namespace text NOT NULL,
 project_id uuid NOT NULL REFERENCES pangu.projects(id),
 created_by uuid REFERENCES pangu.users(id),
 title text NOT NULL DEFAULT '新对话',
 revision integer NOT NULL DEFAULT 1,
 created_at timestamptz NOT NULL DEFAULT now(),
 updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX factory_sessions_project ON pangu.factory_sessions(namespace,project_id,updated_at DESC,id);
CREATE TABLE pangu.factory_turns (
 id uuid PRIMARY KEY,
 session_id uuid NOT NULL REFERENCES pangu.factory_sessions(id) ON DELETE CASCADE,
 sequence integer NOT NULL,
 request_id uuid REFERENCES pangu.submissions(id) ON DELETE SET NULL,
 input jsonb NOT NULL,
 skill_digest text NOT NULL,
 status text NOT NULL CHECK (status IN ('pending','ready','failed','applied','expired')),
 reply jsonb,
 error text NOT NULL DEFAULT '',
 result jsonb,
 created_at timestamptz NOT NULL DEFAULT now(),
 finished_at timestamptz,
 UNIQUE(session_id,sequence)
);
