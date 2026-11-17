ALTER TABLE pangu.approvals DROP CONSTRAINT approvals_status_check;
ALTER TABLE pangu.approvals ADD CONSTRAINT approvals_status_check
 CHECK(status IN ('waiting','approved','rejected','revoked','consumed','expired'));

-- 保留旧审批的过期历史，新审批仅由人工决定改变状态。
UPDATE pangu.approvals SET status='expired'
 WHERE status IN ('waiting','approved') AND expires_at<=clock_timestamp();
ALTER TABLE pangu.approvals DROP COLUMN expires_at;
