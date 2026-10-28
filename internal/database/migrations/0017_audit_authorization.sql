ALTER TABLE pangu.audit_events ADD COLUMN target_id uuid;
ALTER TABLE pangu.audit_events ADD COLUMN role text CHECK(role IN ('manager','operator','viewer'));
