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
	e.GET("/tests", h.getTests)
	e.GET("/tests/:id", h.getTest)
	e.POST("/tests", h.createTest)
	e.DELETE("/tests/:id", h.deleteTest)
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

	c.Response().Header().Set(echo.HeaderLocation, fmt.Sprintf("/tests/%s", res))
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
