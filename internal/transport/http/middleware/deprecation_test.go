package middleware

import (
	"context"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
)

func TestDeprecationMiddleware_SetsHeaders(t *testing.T) {
	sunset := time.Date(2027, 1, 1, 0, 0, 0, 0, time.UTC)

	app := fiber.New()
	app.Use(DeprecationMiddleware(sunset, "/api/v2"))
	app.Get("/tickets/1", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{"ok": true})
	})

	req := httptest.NewRequestWithContext(context.Background(), "GET", "/tickets/1", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}

	if got := resp.Header.Get("Deprecation"); got != "true" {
		t.Errorf("expected Deprecation: true, got %q", got)
	}
	if got := resp.Header.Get("Sunset"); got != "Fri, 01 Jan 2027 00:00:00 GMT" {
		t.Errorf("unexpected Sunset header: %q", got)
	}
	if got := resp.Header.Get("Link"); got != `</api/v2>; rel="successor-version"` {
		t.Errorf("unexpected Link header: %q", got)
	}
}
