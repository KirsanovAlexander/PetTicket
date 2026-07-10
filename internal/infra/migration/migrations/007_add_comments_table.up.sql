CREATE TABLE IF NOT EXISTS ticket_comments (
    id BIGSERIAL PRIMARY KEY,
    ticket_id BIGINT NOT NULL REFERENCES tickets(id) ON DELETE CASCADE,
    user_id BIGINT NOT NULL,
    content TEXT NOT NULL,
    is_internal BOOLEAN NOT NULL DEFAULT false,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_ticket_comments_ticket_created
    ON ticket_comments (ticket_id, created_at);

CREATE INDEX IF NOT EXISTS idx_ticket_comments_public
    ON ticket_comments (ticket_id, created_at)
    WHERE is_internal = false;

COMMENT ON TABLE ticket_comments IS 'Комментарии тикетов — заменяет одиночное поле tickets.comment (см. FEATURE_NEW_COMMENTS)';

-- update_updated_at_column() уже существует (001_init.up.sql, использует её
-- триггер таблицы tickets) — переиспользуем вместо ручного updated_at = NOW()
-- в каждом UPDATE-запросе репозитория.
CREATE TRIGGER update_ticket_comments_updated_at BEFORE UPDATE ON ticket_comments
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();
