INSERT INTO ticket_comments (ticket_id, user_id, content, is_internal, created_at, updated_at)
SELECT id, user_id, comment, false, created_at, updated_at
FROM tickets
WHERE comment IS NOT NULL AND comment <> '';
