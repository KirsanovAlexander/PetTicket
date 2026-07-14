package tickets

import "errors"

var (
	// ErrNotFound возвращается когда тикет не найден
	ErrNotFound = errors.New("ticket not found")

	// ErrInvalidInput возвращается при невалидных входных данных
	ErrInvalidInput = errors.New("invalid input")

	// ErrInvalidStatus возвращается при попытке установить невалидный статус
	ErrInvalidStatus = errors.New("invalid status")

	// ErrInvalidPriority возвращается при попытке установить невалидный приоритет
	ErrInvalidPriority = errors.New("invalid priority")

	// ErrUnauthorized возвращается при попытке доступа к чужому тикету
	ErrUnauthorized = errors.New("unauthorized access")

	// ErrConflict возвращается при конфликте данных
	ErrConflict = errors.New("data conflict")

	// ErrInvalidCursor возвращается при невалидном/повреждённом cursor-токене
	ErrInvalidCursor = errors.New("invalid cursor")

	// ErrTicketAlreadyAssigned возвращается при попытке назначить тикет,
	// который уже назначен на другого саппорта
	ErrTicketAlreadyAssigned = errors.New("ticket already assigned")

	// ErrTicketNotAssigned возвращается при попытке снять назначение с
	// тикета, который никому не назначен
	ErrTicketNotAssigned = errors.New("ticket not assigned")

	// ErrInvalidStatusForAssignment возвращается, когда статус тикета не
	// допускает назначения (resolved/closed/cancelled)
	ErrInvalidStatusForAssignment = errors.New("ticket status does not allow assignment")

	// ErrNotAssignedToYou возвращается при попытке снять назначение с
	// тикета, назначенного на другого саппорта
	ErrNotAssignedToYou = errors.New("ticket not assigned to you")

	// ErrOptimisticLockConflict возвращается, когда AssignWithVersion/
	// UnassignWithVersion не смогли применить UPDATE — версия строки в БД
	// уже не совпадает с той, что видел вызывающий (кто-то опередил)
	ErrOptimisticLockConflict = errors.New("optimistic lock conflict")
)
