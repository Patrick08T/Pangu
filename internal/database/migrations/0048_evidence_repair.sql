CREATE TABLE pangu.evidence_repairs (
    id uuid PRIMARY KEY,
    approval_id uuid NOT NULL REFERENCES pangu.approvals(id),
    subject_hash text NOT NULL,
    body jsonb NOT NULL,
    status text NOT NULL CHECK (status IN ('prepared','approved','superseded','consumed')),
    created_by uuid NOT NULL REFERENCES pangu.users(id),
    created_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (approval_id, subject_hash)
);
CREATE UNIQUE INDEX evidence_repair_current ON pangu.evidence_repairs(approval_id)
WHERE status IN ('prepared','approved','consumed');
