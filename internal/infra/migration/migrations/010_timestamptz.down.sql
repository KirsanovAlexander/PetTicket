ALTER TABLE ticket_comments
    ALTER COLUMN updated_at TYPE TIMESTAMP USING updated_at AT TIME ZONE 'UTC',
    ALTER COLUMN created_at TYPE TIMESTAMP USING created_at AT TIME ZONE 'UTC';

ALTER TABLE notification_outbox
    ALTER COLUMN updated_at TYPE TIMESTAMP USING updated_at AT TIME ZONE 'UTC',
    ALTER COLUMN created_at TYPE TIMESTAMP USING created_at AT TIME ZONE 'UTC',
    ALTER COLUMN sent_at TYPE TIMESTAMP USING sent_at AT TIME ZONE 'UTC',
    ALTER COLUMN next_retry_at TYPE TIMESTAMP USING next_retry_at AT TIME ZONE 'UTC';

ALTER TABLE ticket_history
    ALTER COLUMN created_at TYPE TIMESTAMP USING created_at AT TIME ZONE 'UTC';

ALTER TABLE tickets
    ALTER COLUMN assigned_at TYPE TIMESTAMP USING assigned_at AT TIME ZONE 'UTC',
    ALTER COLUMN last_user_activity_at TYPE TIMESTAMP USING last_user_activity_at AT TIME ZONE 'UTC',
    ALTER COLUMN resolved_at TYPE TIMESTAMP USING resolved_at AT TIME ZONE 'UTC',
    ALTER COLUMN first_response_at TYPE TIMESTAMP USING first_response_at AT TIME ZONE 'UTC',
    ALTER COLUMN resolution_deadline TYPE TIMESTAMP USING resolution_deadline AT TIME ZONE 'UTC',
    ALTER COLUMN response_deadline TYPE TIMESTAMP USING response_deadline AT TIME ZONE 'UTC',
    ALTER COLUMN updated_at TYPE TIMESTAMP USING updated_at AT TIME ZONE 'UTC',
    ALTER COLUMN created_at TYPE TIMESTAMP USING created_at AT TIME ZONE 'UTC';
