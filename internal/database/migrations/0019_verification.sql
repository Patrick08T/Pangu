ALTER TABLE pangu.projects ADD COLUMN verification jsonb NOT NULL DEFAULT '[]'::jsonb CHECK(jsonb_typeof(verification)='array' AND jsonb_array_length(verification)<=32);
