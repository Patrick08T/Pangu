ALTER TABLE pangu.run_cancellations ADD COLUMN last_checked_at timestamptz NOT NULL DEFAULT '-infinity';
CREATE INDEX run_cancellations_fair ON pangu.run_cancellations(namespace, last_checked_at, requested_at)
    WHERE reconciled_at IS NULL;
