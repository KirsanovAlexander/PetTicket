package tickets

import "errors"

var (
	// ErrNotFound возвращается когда тикет не найден
	ErrNotFound = errors.New("ticket not found")

	// ErrInvalidInput возвращается при невалидных входных данных
	ErrInvalidInput = errors.New("invalid input")

	// ErrInvalidStatus возвращается при попытке установить невалидный статус
	ErrInvalidStatus = errors.New("invalid status")

	// ErrUnauthorized возвращается при попытке доступа к чужому тикету
	ErrUnauthorized = errors.New("unauthorized access")

	// ErrConflict возвращается при конфликте данных
	ErrConflict = errors.New("data conflict")
)
