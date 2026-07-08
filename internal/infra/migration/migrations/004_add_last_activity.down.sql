DROP INDEX IF EXISTS idx_tickets_auto_close;
ALTER TABLE tickets DROP COLUMN IF EXISTS last_user_activity_at;
