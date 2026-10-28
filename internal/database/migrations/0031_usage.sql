CREATE TABLE pangu.usage_calls (
 id uuid PRIMARY KEY,
 run_id uuid NOT NULL,
 node_id text NOT NULL,
 node_run_id uuid NOT NULL,
 attempt integer NOT NULL,
 kind text NOT NULL CHECK(kind IN ('model','search','codex')),
 model text NOT NULL,
 price jsonb,
 status text NOT NULL DEFAULT 'unknown' CHECK(status IN ('unknown','reported','completed')),
 input_tokens bigint,
 output_tokens bigint,
 cached_tokens bigint,
 estimate_micros bigint,
 created_at timestamptz NOT NULL DEFAULT now(),
 finished_at timestamptz
);
CREATE INDEX usage_run ON pangu.usage_calls(run_id,created_at,id);
