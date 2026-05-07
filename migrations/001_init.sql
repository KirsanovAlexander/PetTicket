-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS ticket_statuses (
    id INT PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    description TEXT
);

INSERT INTO ticket_statuses (id, name, description) VALUES
(1, 'new', 'Новый тикет'),
(2, 'in_progress', 'В работе'),
(3, 'resolved', 'Решен'),
(4, 'closed', 'Закрыт'),
(5, 'cancelled', 'Отменен')
ON CONFLICT (id) DO NOTHING;

CREATE TABLE IF NOT EXISTS ticket_topics (
    id BIGSERIAL PRIMARY KEY,
    external_id BIGINT NOT NULL UNIQUE,
    title VARCHAR(255) NOT NULL,
    description TEXT
);

CREATE INDEX IF NOT EXISTS idx_ticket_topics_external_id ON ticket_topics(external_id);

INSERT INTO ticket_topics (external_id, title, description) VALUES
(1, 'Не дошло пополнение', 'Проблемы с зачислением депозита'),
(2, 'Не дошел вывод', 'Проблемы с выводом средств'),
(3, 'Ошибка при пополнении', 'Технические ошибки при депозите'),
(4, 'Ошибка при выводе', 'Технические ошибки при выводе')
ON CONFLICT (external_id) DO NOTHING;

CREATE TABLE IF NOT EXISTS tickets (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL,
    topic_id BIGINT NOT NULL,
    status_id INT NOT NULL,
    amount DECIMAL(15,2),
    comment TEXT NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_tickets_user_id ON tickets(user_id);
CREATE INDEX IF NOT EXISTS idx_tickets_topic_id ON tickets(topic_id);
CREATE INDEX IF NOT EXISTS idx_tickets_status_id ON tickets(status_id);
CREATE INDEX IF NOT EXISTS idx_tickets_created_at ON tickets(created_at);

ALTER TABLE tickets ADD CONSTRAINT fk_tickets_topic_id FOREIGN KEY (topic_id) REFERENCES ticket_topics(id);
ALTER TABLE tickets ADD CONSTRAINT fk_tickets_status_id FOREIGN KEY (status_id) REFERENCES ticket_statuses(id);

CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = CURRENT_TIMESTAMP;
    RETURN NEW;
END;
$$ language 'plpgsql';

CREATE TRIGGER update_tickets_updated_at BEFORE UPDATE ON tickets
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TABLE IF NOT EXISTS ticket_history (
    id BIGSERIAL PRIMARY KEY,
    ticket_id BIGINT NOT NULL,
    user_id BIGINT NOT NULL,
    action VARCHAR(255) NOT NULL,
    old_value TEXT,
    new_value TEXT,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_ticket_history_ticket_id ON ticket_history(ticket_id);
CREATE INDEX IF NOT EXISTS idx_ticket_history_created_at ON ticket_history(created_at);

ALTER TABLE ticket_history ADD CONSTRAINT fk_ticket_history_ticket_id 
    FOREIGN KEY (ticket_id) REFERENCES tickets(id) ON DELETE CASCADE;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS ticket_history;
DROP TABLE IF EXISTS tickets;
DROP TRIGGER IF EXISTS update_tickets_updated_at ON tickets;
DROP FUNCTION IF EXISTS update_updated_at_column();
DROP TABLE IF EXISTS ticket_topics;
DROP TABLE IF EXISTS ticket_statuses;
-- +goose StatementEnd
