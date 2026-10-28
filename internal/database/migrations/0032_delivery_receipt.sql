ALTER TABLE pangu.deliveries ADD COLUMN submission_hash text;
UPDATE pangu.deliveries d SET submission_hash=s.payload_hash FROM pangu.submissions s WHERE s.id=d.request_id;
