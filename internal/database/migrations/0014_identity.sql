CREATE TABLE pangu.users (
    id uuid PRIMARY KEY,
    namespace text NOT NULL,
    app_id text NOT NULL,
    tenant_key text NOT NULL,
    open_id text NOT NULL,
    name text NOT NULL DEFAULT '',
    is_admin boolean NOT NULL DEFAULT false,
    disabled boolean NOT NULL DEFAULT false,
    created_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE(namespace,app_id,tenant_key,open_id)
);
CREATE TABLE pangu.memberships (
    user_id uuid NOT NULL REFERENCES pangu.users(id),
    project_id uuid NOT NULL REFERENCES pangu.projects(id),
    role text NOT NULL CHECK(role IN ('manager','operator','viewer')),
    enabled boolean NOT NULL DEFAULT true,
    PRIMARY KEY(user_id,project_id)
);
CREATE TABLE pangu.login_states (
    state_hash text PRIMARY KEY,
    browser_hash text NOT NULL,
    namespace text NOT NULL,
    app_id text NOT NULL,
    redirect_uri text NOT NULL,
    verifier text NOT NULL,
    expires_at timestamptz NOT NULL
);
CREATE TABLE pangu.sessions (
    token_hash text PRIMARY KEY,
    user_id uuid NOT NULL REFERENCES pangu.users(id),
    expires_at timestamptz NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX sessions_user ON pangu.sessions(user_id);
CREATE TABLE pangu.api_tokens (
    id uuid PRIMARY KEY,
    token_hash text NOT NULL UNIQUE,
    user_id uuid NOT NULL REFERENCES pangu.users(id),
    project_id uuid NOT NULL REFERENCES pangu.projects(id),
    name text NOT NULL,
    scopes text[] NOT NULL,
    expires_at timestamptz NOT NULL,
    revoked_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX api_tokens_user ON pangu.api_tokens(user_id);
