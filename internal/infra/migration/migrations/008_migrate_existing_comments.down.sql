DELETE FROM ticket_comments tc
USING tickets t
WHERE tc.ticket_id = t.id
  AND tc.content = t.comment
  AND tc.created_at = t.created_at
  AND tc.is_internal = false;
