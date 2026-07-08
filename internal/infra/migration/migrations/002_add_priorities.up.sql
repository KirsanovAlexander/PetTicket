CREATE TABLE IF NOT EXISTS ticket_priorities (
    id INT PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    display_name VARCHAR(255) NOT NULL,
    level INT NOT NULL
);

INSERT INTO ticket_priorities (id, name, display_name, level) VALUES
(1, 'low', 'Low', 1),
(2, 'medium', 'Medium', 2),
(3, 'high', 'High', 3),
(4, 'critical', 'Critical', 4)
ON CONFLICT (id) DO NOTHING;

ALTER TABLE tickets ADD COLUMN IF NOT EXISTS priority_id INT NOT NULL DEFAULT 2;

ALTER TABLE tickets DROP CONSTRAINT IF EXISTS fk_tickets_priority_id;
ALTER TABLE tickets ADD CONSTRAINT fk_tickets_priority_id
    FOREIGN KEY (priority_id) REFERENCES ticket_priorities(id);

CREATE INDEX IF NOT EXISTS idx_tickets_priority_id ON tickets(priority_id);
