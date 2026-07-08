DROP INDEX IF EXISTS idx_tickets_resolution_deadline_pending;
DROP INDEX IF EXISTS idx_tickets_response_deadline_pending;

ALTER TABLE tickets DROP COLUMN IF EXISTS resolved_at;
ALTER TABLE tickets DROP COLUMN IF EXISTS first_response_at;
ALTER TABLE tickets DROP COLUMN IF EXISTS resolution_deadline;
ALTER TABLE tickets DROP COLUMN IF EXISTS response_deadline;

DROP TABLE IF EXISTS sla_rules;
