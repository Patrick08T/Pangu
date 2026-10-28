CREATE TABLE pangu.agent_trials (
 request_id uuid PRIMARY KEY REFERENCES pangu.submissions(id) ON DELETE CASCADE,
 object_id uuid NOT NULL REFERENCES pangu.catalog_objects(id),
 revision integer NOT NULL CHECK (revision > 0)
);
CREATE INDEX agent_trials_object ON pangu.agent_trials(object_id,request_id);
