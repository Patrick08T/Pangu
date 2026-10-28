ALTER TABLE pangu.research_reports ADD COLUMN metadata jsonb;
CREATE TABLE pangu.claim_reviews (
 report_id uuid NOT NULL REFERENCES pangu.research_reports(id) ON DELETE CASCADE,
 claim_id text NOT NULL,
 actor_id uuid NOT NULL REFERENCES pangu.users(id),
 subject_hash text NOT NULL,
 revision integer NOT NULL DEFAULT 1,
 verdict text NOT NULL CHECK(verdict IN ('supported','partial','unsupported','outdated')),
 reason text NOT NULL,
 updated_at timestamptz NOT NULL DEFAULT now(),
 PRIMARY KEY(report_id,claim_id)
);
