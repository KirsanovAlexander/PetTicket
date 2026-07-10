-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS ticket_comments (
    id BIGSERIAL PRIMARY KEY,
    ticket_id BIGINT NOT NULL REFERENCES tickets(id) ON DELETE CASCADE,
    user_id BIGINT NOT NULL,
    content TEXT NOT NULL,
    is_internal BOOLEAN NOT NULL DEFAULT false,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW()
);

-- Основной паттерн доступа: все комментарии тикета в хронологическом
-- порядке (GetByTicketID) или последний (GetLastByTicketID DESC LIMIT 1) —
-- один составной индекс покрывает оба случая.
CREATE INDEX IF NOT EXISTS idx_ticket_comments_ticket_created
    ON ticket_comments (ticket_id, created_at);

-- Частый случай — публичный тред без internal-заметок (например, для
-- отображения клиенту). Partial index меньше основного, если internal-заметок
-- много.
CREATE INDEX IF NOT EXISTS idx_ticket_comments_public
    ON ticket_comments (ticket_id, created_at)
    WHERE is_internal = false;

COMMENT ON TABLE ticket_comments IS 'Комментарии тикетов — заменяет одиночное поле tickets.comment (см. FEATURE_NEW_COMMENTS)';

-- update_updated_at_column() уже существует (001_init.sql, использует её
-- триггер таблицы tickets) — переиспользуем вместо ручного updated_at = NOW()
-- в каждом UPDATE-запросе репозитория. Так updated_at гарантированно
-- обновляется при любом UPDATE строки, даже случайном сыром SQL в обход
-- CommentsRepository.
CREATE TRIGGER update_ticket_comments_updated_at BEFORE UPDATE ON ticket_comments
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS ticket_comments;
-- +goose StatementEnd
