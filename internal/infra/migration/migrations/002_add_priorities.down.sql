DROP INDEX IF EXISTS idx_tickets_priority_id;

ALTER TABLE tickets DROP CONSTRAINT IF EXISTS fk_tickets_priority_id;
ALTER TABLE tickets DROP COLUMN IF EXISTS priority_id;

DROP TABLE IF EXISTS ticket_priorities;
