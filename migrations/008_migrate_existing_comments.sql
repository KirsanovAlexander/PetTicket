-- +goose Up
-- +goose StatementBegin
-- Переносим текущее значение tickets.comment в ticket_comments — по одной
-- строке на тикет (единственный "комментарий", который был доступен в
-- старой модели). Автора комментария в старой модели не хранили отдельно —
-- используем tickets.user_id (владелец тикета) как наиболее разумное
-- приближение. Пустые комментарии не переносим — переносить нечего.
INSERT INTO ticket_comments (ticket_id, user_id, content, is_internal, created_at, updated_at)
SELECT id, user_id, comment, false, created_at, updated_at
FROM tickets
WHERE comment IS NOT NULL AND comment <> '';
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
-- Best-effort откат: data-миграцию нельзя откатить со стопроцентной
-- точностью (если после неё добавили новые комментарии — они не должны
-- быть случайно задеты). Удаляем только строки, которые точно совпадают по
-- ticket_id/content/created_at с текущим tickets.comment и не являются
-- internal — то есть именно те, что создала миграция выше.
DELETE FROM ticket_comments tc
USING tickets t
WHERE tc.ticket_id = t.id
  AND tc.content = t.comment
  AND tc.created_at = t.created_at
  AND tc.is_internal = false;
-- +goose StatementEnd
