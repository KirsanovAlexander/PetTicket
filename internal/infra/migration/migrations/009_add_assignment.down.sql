DROP INDEX IF EXISTS idx_tickets_unassigned;
DROP INDEX IF EXISTS idx_tickets_assigned_to;
ALTER TABLE tickets
    DROP COLUMN version,
    DROP COLUMN assigned_at,
    DROP COLUMN assigned_to;
