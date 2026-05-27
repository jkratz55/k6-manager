package internal

import (
	"net/http"

	"github.com/labstack/echo/v5"
)

type HealthHandler struct{}

func (h *HealthHandler) RegisterRoutes(e *echo.Echo) {
	group := e.Group("/healthz")
	group.GET("/live", h.live)
	group.GET("/ready", h.ready)
}

func (h *HealthHandler) live(c *echo.Context) error {
	return c.JSON(http.StatusOK, map[string]string{
		"status": "up",
	})
}

func (h *HealthHandler) ready(c *echo.Context) error {
	return c.JSON(http.StatusOK, map[string]string{
		"status": "up",
	})
}
