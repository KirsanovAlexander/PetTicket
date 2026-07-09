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
)
