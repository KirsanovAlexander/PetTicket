-- Все временные колонки были TIMESTAMP (без часового пояса). Значения писал
-- Go-код (time.Now() в таймзоне процесса-клиента), а сравнивались они внутри
-- Postgres через NOW() (в таймзоне сессии, по умолчанию UTC) — если клиент и
-- БД не в одной таймзоне, "TIMESTAMP <= NOW()" даёт неверный результат
-- (next_retry_at в outbox никогда не наступает, SLA-нарушения находятся не
-- вовремя). В docker-compose это молчало, потому что и app, и postgres по
-- умолчанию оба в UTC, но воспроизводится стабильно, если клиент (например,
-- дев-машина не в UTC) пишет время напрямую. USING col AT TIME ZONE 'UTC'
-- трактует уже сохранённые "голые" значения как UTC — ровно то, чем они и
-- были при исходной DEFAULT NOW()/CURRENT_TIMESTAMP на UTC-сервере.
ALTER TABLE tickets
    ALTER COLUMN created_at TYPE TIMESTAMPTZ USING created_at AT TIME ZONE 'UTC',
    ALTER COLUMN updated_at TYPE TIMESTAMPTZ USING updated_at AT TIME ZONE 'UTC',
    ALTER COLUMN response_deadline TYPE TIMESTAMPTZ USING response_deadline AT TIME ZONE 'UTC',
    ALTER COLUMN resolution_deadline TYPE TIMESTAMPTZ USING resolution_deadline AT TIME ZONE 'UTC',
    ALTER COLUMN first_response_at TYPE TIMESTAMPTZ USING first_response_at AT TIME ZONE 'UTC',
    ALTER COLUMN resolved_at TYPE TIMESTAMPTZ USING resolved_at AT TIME ZONE 'UTC',
    ALTER COLUMN last_user_activity_at TYPE TIMESTAMPTZ USING last_user_activity_at AT TIME ZONE 'UTC',
    ALTER COLUMN assigned_at TYPE TIMESTAMPTZ USING assigned_at AT TIME ZONE 'UTC';

ALTER TABLE ticket_history
    ALTER COLUMN created_at TYPE TIMESTAMPTZ USING created_at AT TIME ZONE 'UTC';

ALTER TABLE notification_outbox
    ALTER COLUMN next_retry_at TYPE TIMESTAMPTZ USING next_retry_at AT TIME ZONE 'UTC',
    ALTER COLUMN sent_at TYPE TIMESTAMPTZ USING sent_at AT TIME ZONE 'UTC',
    ALTER COLUMN created_at TYPE TIMESTAMPTZ USING created_at AT TIME ZONE 'UTC',
    ALTER COLUMN updated_at TYPE TIMESTAMPTZ USING updated_at AT TIME ZONE 'UTC';

ALTER TABLE ticket_comments
    ALTER COLUMN created_at TYPE TIMESTAMPTZ USING created_at AT TIME ZONE 'UTC',
    ALTER COLUMN updated_at TYPE TIMESTAMPTZ USING updated_at AT TIME ZONE 'UTC';
