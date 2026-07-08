-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS sla_rules (
    id SERIAL PRIMARY KEY,
    topic_id BIGINT NOT NULL,
    priority_id INT NOT NULL,
    response_time_minutes INT NOT NULL,
    resolution_time_minutes INT NOT NULL,
    UNIQUE (topic_id, priority_id)
);

INSERT INTO sla_rules (topic_id, priority_id, response_time_minutes, resolution_time_minutes) VALUES
(1, 1, 120, 1440),
(1, 2, 60, 720),
(1, 3, 30, 480),
(1, 4, 15, 240),
(2, 1, 120, 1440),
(2, 2, 60, 720),
(2, 3, 30, 480),
(2, 4, 15, 240),
(3, 1, 120, 1440),
(3, 2, 60, 720),
(3, 3, 30, 480),
(3, 4, 15, 240),
(4, 1, 120, 1440),
(4, 2, 60, 720),
(4, 3, 30, 480),
(4, 4, 15, 240)
ON CONFLICT (topic_id, priority_id) DO NOTHING;

ALTER TABLE tickets ADD COLUMN IF NOT EXISTS response_deadline TIMESTAMP;
ALTER TABLE tickets ADD COLUMN IF NOT EXISTS resolution_deadline TIMESTAMP;
ALTER TABLE tickets ADD COLUMN IF NOT EXISTS first_response_at TIMESTAMP;
ALTER TABLE tickets ADD COLUMN IF NOT EXISTS resolved_at TIMESTAMP;

CREATE INDEX IF NOT EXISTS idx_tickets_response_deadline_pending
    ON tickets(response_deadline) WHERE first_response_at IS NULL;

CREATE INDEX IF NOT EXISTS idx_tickets_resolution_deadline_pending
    ON tickets(resolution_deadline) WHERE resolved_at IS NULL;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX IF EXISTS idx_tickets_resolution_deadline_pending;
DROP INDEX IF EXISTS idx_tickets_response_deadline_pending;

ALTER TABLE tickets DROP COLUMN IF EXISTS resolved_at;
ALTER TABLE tickets DROP COLUMN IF EXISTS first_response_at;
ALTER TABLE tickets DROP COLUMN IF EXISTS resolution_deadline;
ALTER TABLE tickets DROP COLUMN IF EXISTS response_deadline;

DROP TABLE IF EXISTS sla_rules;
-- +goose StatementEnd
