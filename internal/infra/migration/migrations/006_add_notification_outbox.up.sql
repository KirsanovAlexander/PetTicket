CREATE TABLE IF NOT EXISTS notification_outbox (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL,
    ticket_id BIGINT NOT NULL,
    notification_type VARCHAR(50) NOT NULL,
    payload JSONB NOT NULL DEFAULT '{}',
    status VARCHAR(20) NOT NULL DEFAULT 'pending',
    attempts INT NOT NULL DEFAULT 0,
    max_attempts INT NOT NULL DEFAULT 5,
    next_retry_at TIMESTAMP NOT NULL DEFAULT NOW(),
    sent_at TIMESTAMP,
    error_message TEXT,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW()
);

ALTER TABLE notification_outbox
    ADD CONSTRAINT fk_notification_outbox_ticket_id
    FOREIGN KEY (ticket_id) REFERENCES tickets(id) ON DELETE CASCADE;

CREATE INDEX IF NOT EXISTS idx_notification_outbox_pending
    ON notification_outbox (next_retry_at)
    WHERE status = 'pending';

CREATE INDEX IF NOT EXISTS idx_notification_outbox_ticket_id ON notification_outbox(ticket_id);

COMMENT ON TABLE notification_outbox IS 'Transactional outbox: уведомления, создаваемые в той же транзакции, что и бизнес-изменение';
COMMENT ON INDEX idx_notification_outbox_pending IS 'Для FindPending: только pending-записи, готовые к retry';
