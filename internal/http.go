package internal

import (
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
		return err // TODO: proper error handling
	}
	return c.JSON(http.StatusOK, res)
}

func (h *Handler) getTest(c *echo.Context) error {
	panic("implement me")
}

func (h *Handler) createTest(c *echo.Context) error {
	var req CreateTestRequest
	err := c.Bind(&req)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadGateway, err.Error()) // todo: proper error handling
	}

	err = c.Validate(req)
	if err != nil {
		return echo.NewHTTPError(http.StatusUnprocessableEntity, err.Error()) // todo: proper error handling
	}

	res, err := h.service.CreateTest(c.Request().Context(), req)
	if err != nil {
		Logger().Error("Failed to create test", slog.Any("error", err))
		return err // todo: proper error handling
	}

	return c.JSON(http.StatusCreated, res) // todo: set Location header
}

func (h *Handler) deleteTest(c *echo.Context) error {
	panic("implement me")
}
