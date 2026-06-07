package internal

import (
	"fmt"
	"log/slog"
	"net/http"

	"github.com/labstack/echo/v5"
)

type Handler struct {
	service *K6Service
}

func NewHandler(service *K6Service) *Handler {
	return &Handler{service: service}
}

func (h *Handler) RegisterRoutes(e *echo.Echo) {
	api := e.Group("/api")
	api.GET("/tests", h.getTests)
	api.GET("/tests/:id", h.getTest)
	api.POST("/tests", h.createTest)
	api.POST("/tests/:id/rerun", h.rerunTest)
	api.DELETE("/tests/:id", h.deleteTest)
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
	return c.JSON(http.StatusCreated, map[string]string{"id": res})
}

func (h *Handler) rerunTest(c *echo.Context) error {
	id := c.Param("id")
	res, err := h.service.ReRunTest(c.Request().Context(), id)
	if err != nil {
		Logger().Error("Failed to rerun test", slog.String("id", id), slog.Any("error", err))
		return InternalServerError()
	}

	c.Response().Header().Set(echo.HeaderLocation, fmt.Sprintf("/api/tests/%s", res))
	return c.JSON(http.StatusCreated, map[string]string{"id": res})
}

func (h *Handler) deleteTest(c *echo.Context) error {
	id := c.Param("id")
	if err := h.service.DeleteTest(c.Request().Context(), id); err != nil {
		Logger().Error("Failed to delete test", slog.String("id", id), slog.Any("error", err))
		return InternalServerError()
	}
	return c.NoContent(http.StatusNoContent)
}
