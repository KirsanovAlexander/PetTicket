ALTER TABLE tickets
    ADD COLUMN assigned_to BIGINT,
    ADD COLUMN assigned_at TIMESTAMP,
    ADD COLUMN version INTEGER NOT NULL DEFAULT 1;

CREATE INDEX IF NOT EXISTS idx_tickets_assigned_to ON tickets(assigned_to) WHERE assigned_to IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_tickets_unassigned ON tickets(status_id, created_at DESC)
    WHERE assigned_to IS NULL AND status_id IN (1, 2);
