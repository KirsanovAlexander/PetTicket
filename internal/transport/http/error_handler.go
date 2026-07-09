package http

import (
	"errors"

	"pet-ticket/internal/app/tickets"
	"pet-ticket/internal/transport/http/dto"

	"github.com/gofiber/fiber/v2"
	"github.com/rs/zerolog"
)

// NewErrorHandler создаёт централизованный обработчик ошибок
func NewErrorHandler(logger zerolog.Logger, env string) fiber.ErrorHandler {
	return func(c *fiber.Ctx, err error) error {
		code := fiber.StatusInternalServerError
		errorCode := "INTERNAL_SERVER_ERROR"
		message := "internal server error"

		// Fiber errors
		var fiberErr *fiber.Error
		if errors.As(err, &fiberErr) {
			code = fiberErr.Code
			message = fiberErr.Message
			errorCode = httpStatusToErrorCode(code)
		}

		// Domain errors
		switch {
		case errors.Is(err, tickets.ErrNotFound):
			code = fiber.StatusNotFound
			errorCode = "NOT_FOUND"
			message = "ticket not found"
		case errors.Is(err, tickets.ErrInvalidInput):
			code = fiber.StatusBadRequest
			errorCode = "INVALID_INPUT"
			message = "request validation failed"
		case errors.Is(err, tickets.ErrInvalidStatus):
			code = fiber.StatusBadRequest
			errorCode = "INVALID_STATUS"
			message = "status value is not valid"
		case errors.Is(err, tickets.ErrUnauthorized):
			code = fiber.StatusForbidden
			errorCode = "UNAUTHORIZED"
			message = "you don't have permission to access this resource"
		case errors.Is(err, tickets.ErrInvalidCursor):
			code = fiber.StatusBadRequest
			errorCode = "INVALID_CURSOR"
			message = "cursor is invalid or malformed"
		}

		// Request ID для трейсинга
		requestID := ""
		if rid := c.Locals("requestId"); rid != nil {
			requestID = rid.(string)
		}

		// Логирование
		if code >= 500 {
			logger.Error().
				Err(err).
				Int("status", code).
				Str("errorCode", errorCode).
				Str("requestId", requestID).
				Msg("http error")
		} else {
			logger.Warn().
				Err(err).
				Int("status", code).
				Str("errorCode", errorCode).
				Str("requestId", requestID).
				Msg("http error")
		}

		// В production не показываем детали внутренних ошибок
		if env == "production" && code >= 500 {
			message = "internal server error"
		}

		response := dto.ErrorResponse{
			Error: dto.ErrorDetail{
				Code:      errorCode,
				Message:   message,
				RequestID: requestID,
			},
		}

		return c.Status(code).JSON(response)
	}
}

// httpStatusToErrorCode конвертирует HTTP статус в error code
func httpStatusToErrorCode(status int) string {
	switch status {
	case fiber.StatusBadRequest:
		return "BAD_REQUEST"
	case fiber.StatusUnauthorized:
		return "UNAUTHORIZED"
	case fiber.StatusForbidden:
		return "FORBIDDEN"
	case fiber.StatusNotFound:
		return "NOT_FOUND"
	case fiber.StatusMethodNotAllowed:
		return "METHOD_NOT_ALLOWED"
	case fiber.StatusConflict:
		return "CONFLICT"
	case fiber.StatusUnprocessableEntity:
		return "UNPROCESSABLE_ENTITY"
	case fiber.StatusTooManyRequests:
		return "TOO_MANY_REQUESTS"
	case fiber.StatusInternalServerError:
		return "INTERNAL_SERVER_ERROR"
	case fiber.StatusServiceUnavailable:
		return "SERVICE_UNAVAILABLE"
	default:
		return "UNKNOWN_ERROR"
	}
}
