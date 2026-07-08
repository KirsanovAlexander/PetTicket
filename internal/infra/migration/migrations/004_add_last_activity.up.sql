ALTER TABLE tickets
ADD COLUMN last_user_activity_at TIMESTAMP DEFAULT NOW() NOT NULL;

UPDATE tickets
SET last_user_activity_at = updated_at
WHERE last_user_activity_at IS NULL;

CREATE INDEX idx_tickets_auto_close
ON tickets(status_id, last_user_activity_at)
WHERE status_id = 3;

COMMENT ON COLUMN tickets.last_user_activity_at IS 'Время последней активности пользователя (для автозакрытия)';
