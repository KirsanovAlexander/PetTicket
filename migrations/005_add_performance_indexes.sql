-- +goose Up
-- +goose StatementBegin
-- Композитный индекс для cursor-пагинации: WHERE (created_at, id) < ($1, $2) ORDER BY created_at, id
CREATE INDEX IF NOT EXISTS idx_tickets_created_at_id ON tickets(created_at, id);

-- Типичные запросы саппорта: тикеты пользователя/темы с фильтром по статусу
CREATE INDEX IF NOT EXISTS idx_tickets_user_status ON tickets(user_id, status_id);
CREATE INDEX IF NOT EXISTS idx_tickets_topic_status ON tickets(topic_id, status_id);

-- Список тикетов по статусу, отсортированный по свежести (очередь саппорта)
CREATE INDEX IF NOT EXISTS idx_tickets_status_created_at ON tickets(status_id, created_at DESC);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX IF EXISTS idx_tickets_status_created_at;
DROP INDEX IF EXISTS idx_tickets_topic_status;
DROP INDEX IF EXISTS idx_tickets_user_status;
DROP INDEX IF EXISTS idx_tickets_created_at_id;
-- +goose StatementEnd
