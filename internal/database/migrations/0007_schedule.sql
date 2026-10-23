CREATE TABLE pangu.schedules (
    id uuid PRIMARY KEY,
    namespace text NOT NULL,
    project_id uuid NOT NULL REFERENCES pangu.projects(id),
    name text NOT NULL,
    expression text NOT NULL,
    workflow jsonb NOT NULL,
    revision integer NOT NULL DEFAULT 1,
    paused boolean NOT NULL DEFAULT false,
    deleted boolean NOT NULL DEFAULT false,
    hatchet_id uuid,
    active_slot timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE(namespace,name)
);
CREATE TABLE pangu.schedule_fires (
    schedule_id uuid NOT NULL REFERENCES pangu.schedules(id),
    slot timestamptz NOT NULL,
    snapshot jsonb NOT NULL,
    status text NOT NULL CHECK(status IN ('pending','accepted','skipped')),
    reason text NOT NULL DEFAULT '',
    request_id uuid REFERENCES pangu.submissions(id),
    created_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY(schedule_id,slot)
);
