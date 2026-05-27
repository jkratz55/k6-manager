package internal

import (
	"fmt"
	"io/fs"
	"log/slog"
	"net/http"
	"strings"

	"github.com/labstack/echo/v5"

	"github.com/jkratz55/k6-manager/frontend"
)

type Handler struct {
	service *K6Service
}

func NewHandler(service *K6Service) *Handler {
	return &Handler{service: service}
}

func (h *Handler) RegisterRoutes(e *echo.Echo) {
	api := e.Group("/api")
	api.GET("/health", h.health)
	api.GET("/tests", h.getTests)
	api.GET("/tests/:id", h.getTest)
	api.POST("/tests", h.createTest)
	api.DELETE("/tests/:id", h.deleteTest)

	distFS, err := fs.Sub(frontend.DistDir, "dist")
	if err != nil {
		panic(err)
	}
	fileServer := http.FileServer(http.FS(distFS))

	e.GET("/*", func(c *echo.Context) error {
		p := c.Request().URL.Path
		if strings.HasPrefix(p, "/api") {
			return echo.ErrNotFound
		}
		if p == "/" {
			return c.FileFS("index.html", distFS)
		}

		name := strings.TrimPrefix(p, "/")
		_, err := fs.Stat(distFS, name)
		if err == nil {
			return echo.WrapHandler(fileServer)(c)
		}
		return c.FileFS("index.html", distFS)
	})
}

func (h *Handler) health(c *echo.Context) error {
	return c.NoContent(http.StatusOK)
}

func (h *Handler) getTests(c *echo.Context) error {
	res, err := h.service.GetTests(c.Request().Context())
	if err != nil {
		Logger().Error("Failed to list tests", slog.Any("error", err))
		return InternalServerError()
	}
	return c.JSON(http.StatusOK, res)
}

func (h *Handler) getTest(c *echo.Context) error {
	id := c.Param("id")
	res, err := h.service.GetTest(c.Request().Context(), id)
	if err != nil {
		Logger().Error("Failed to get test", slog.String("id", id), slog.Any("error", err))
		return InternalServerError()
	}
	return c.JSON(http.StatusOK, res)
}

func (h *Handler) createTest(c *echo.Context) error {
	var req CreateTestRequest
	if err := c.Bind(&req); err != nil {
		return BadRequest()
	}

	if err := c.Validate(req); err != nil {
		return UnprocessableEntity(err)
	}

	res, err := h.service.CreateTest(c.Request().Context(), req)
	if err != nil {
		Logger().Error("Failed to create test", slog.Any("error", err))
		return InternalServerError()
	}

	c.Response().Header().Set(echo.HeaderLocation, fmt.Sprintf("/api/tests/%s", res))
	return c.NoContent(http.StatusCreated)
}

func (h *Handler) deleteTest(c *echo.Context) error {
	id := c.Param("id")
	if err := h.service.DeleteTest(c.Request().Context(), id); err != nil {
		Logger().Error("Failed to delete test", slog.String("id", id), slog.Any("error", err))
		return InternalServerError()
	}
	return c.NoContent(http.StatusNoContent)
}
