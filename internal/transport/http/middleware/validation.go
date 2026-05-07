package middleware

import (
	"github.com/go-playground/validator/v10"
	"github.com/gofiber/fiber/v2"
)

var validate = validator.New()

// ValidateBody валидирует body запроса и сохраняет в context
func ValidateBody[T any]() fiber.Handler {
	return func(c *fiber.Ctx) error {
		var body T

		// Парсинг body
		if err := c.BodyParser(&body); err != nil {
			return fiber.NewError(fiber.StatusBadRequest, "invalid json body")
		}

		// Валидация
		if err := validate.Struct(body); err != nil {
			validationErrors := err.(validator.ValidationErrors)
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": fiber.Map{
					"code":      "VALIDATION_ERROR",
					"message":   "request validation failed",
					"requestId": c.Locals("requestId"),
					"details":   formatValidationErrors(validationErrors),
				},
			})
		}

		// Сохраняем валидное body в контекст
		c.Locals("validatedBody", body)
		return c.Next()
	}
}

// ValidateQuery валидирует query параметры
func ValidateQuery[T any]() fiber.Handler {
	return func(c *fiber.Ctx) error {
		var query T

		// Парсинг query
		if err := c.QueryParser(&query); err != nil {
			return fiber.NewError(fiber.StatusBadRequest, "invalid query parameters")
		}

		// Валидация
		if err := validate.Struct(query); err != nil {
			validationErrors := err.(validator.ValidationErrors)
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": fiber.Map{
					"code":      "VALIDATION_ERROR",
					"message":   "query validation failed",
					"requestId": c.Locals("requestId"),
					"details":   formatValidationErrors(validationErrors),
				},
			})
		}

		c.Locals("validatedQuery", query)
		return c.Next()
	}
}

// GetValidatedBody извлекает валидное body из контекста
func GetValidatedBody[T any](c *fiber.Ctx) T {
	return c.Locals("validatedBody").(T)
}

// GetValidatedQuery извлекает валидные query параметры из контекста
func GetValidatedQuery[T any](c *fiber.Ctx) T {
	return c.Locals("validatedQuery").(T)
}

// formatValidationErrors форматирует ошибки валидации в читаемый вид
func formatValidationErrors(errs validator.ValidationErrors) []fiber.Map {
	errors := make([]fiber.Map, 0, len(errs))
	for _, err := range errs {
		errors = append(errors, fiber.Map{
			"field":   err.Field(),
			"tag":     err.Tag(),
			"value":   err.Value(),
			"message": getErrorMessage(err),
		})
	}
	return errors
}

// getErrorMessage возвращает человекочитаемое сообщение для ошибки валидации
func getErrorMessage(err validator.FieldError) string {
	switch err.Tag() {
	case "required":
		return err.Field() + " is required"
	case "email":
		return err.Field() + " must be a valid email"
	case "min":
		return err.Field() + " must be at least " + err.Param()
	case "max":
		return err.Field() + " must be at most " + err.Param()
	case "gte":
		return err.Field() + " must be greater than or equal to " + err.Param()
	case "lte":
		return err.Field() + " must be less than or equal to " + err.Param()
	default:
		return err.Field() + " failed validation: " + err.Tag()
	}
}
