-- +goose Up
-- +goose StatementBegin
ALTER TABLE tickets
ADD COLUMN IF NOT EXISTS last_user_activity_at TIMESTAMP DEFAULT NOW() NOT NULL;

UPDATE tickets
SET last_user_activity_at = updated_at
WHERE last_user_activity_at IS NULL;

CREATE INDEX IF NOT EXISTS idx_tickets_auto_close
ON tickets(status_id, last_user_activity_at)
WHERE status_id = 3;

COMMENT ON COLUMN tickets.last_user_activity_at IS 'Время последней активности пользователя (для автозакрытия)';
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX IF EXISTS idx_tickets_auto_close;
ALTER TABLE tickets DROP COLUMN IF EXISTS last_user_activity_at;
-- +goose StatementEnd
