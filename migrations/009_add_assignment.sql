-- +goose Up
-- +goose StatementBegin
-- assigned_to/assigned_at — кто и когда взял тикет в работу. version — токен
-- optimistic locking: AssignWithVersion/UnassignWithVersion в репозитории
-- обновляют строку только через WHERE version = $N, поэтому при гонке
-- нескольких саппортов за один тикет ровно один UPDATE проходит, остальные
-- получают rowsAffected=0 -> ErrOptimisticLockConflict.
-- TIMESTAMP (без TZ), не TIMESTAMPTZ — остальные временные колонки этой
-- таблицы (created_at, updated_at, response_deadline, resolved_at и т.д.,
-- см. 001_init.sql/003_add_sla.sql) все без timezone; заводить assigned_at
-- единственной TZ-aware колонкой значило бы молчаливое рассогласование
-- типов при сравнениях/джойнах с остальными датами тикета.
ALTER TABLE tickets
    ADD COLUMN assigned_to BIGINT,
    ADD COLUMN assigned_at TIMESTAMP,
    ADD COLUMN version INTEGER NOT NULL DEFAULT 1;

-- Partial-индексы вместо обычных: выборки саппорта всегда однобокие —
-- "мои тикеты" (assigned_to = X) или "свободная очередь" (assigned_to IS
-- NULL), а не произвольные значения колонки. Partial-индекс не тратит место
-- на строки, которые под него не подпадают, и держит обе выборки быстрыми
-- независимо от того, какая доля тикетов уже разобрана.
CREATE INDEX IF NOT EXISTS idx_tickets_assigned_to ON tickets(assigned_to) WHERE assigned_to IS NOT NULL;

-- Очередь свободных тикетов: только активные статусы (new/in_progress —
-- ровно то, что CanBeAssigned() считает назначаемым), отсортированы по
-- свежести. created_at DESC в самом индексе — ORDER BY очереди саппорта
-- не требует отдельной сортировки в рантайме.
CREATE INDEX IF NOT EXISTS idx_tickets_unassigned ON tickets(status_id, created_at DESC)
    WHERE assigned_to IS NULL AND status_id IN (1, 2);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX IF EXISTS idx_tickets_unassigned;
DROP INDEX IF EXISTS idx_tickets_assigned_to;
ALTER TABLE tickets
    DROP COLUMN version,
    DROP COLUMN assigned_at,
    DROP COLUMN assigned_to;
-- +goose StatementEnd
