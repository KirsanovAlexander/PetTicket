CREATE INDEX IF NOT EXISTS idx_tickets_created_at_id ON tickets(created_at, id);
CREATE INDEX IF NOT EXISTS idx_tickets_user_status ON tickets(user_id, status_id);
CREATE INDEX IF NOT EXISTS idx_tickets_topic_status ON tickets(topic_id, status_id);
CREATE INDEX IF NOT EXISTS idx_tickets_status_created_at ON tickets(status_id, created_at DESC);
