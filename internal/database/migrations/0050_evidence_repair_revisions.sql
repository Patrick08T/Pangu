-- 允许用户在新修订后重新提出相同内容；当前方案仍由部分唯一索引约束。
ALTER TABLE pangu.evidence_repairs DROP CONSTRAINT evidence_repairs_approval_id_subject_hash_key;
