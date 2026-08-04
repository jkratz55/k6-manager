package internal

import (
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"strings"

	"github.com/labstack/echo/v5"
)

const envVarFormPrefix = "envVars["

// parseFormEnvVars extracts environment variables submitted as bracket-keyed
// form fields (e.g. "envVars[FOO]=bar") into a map. Echo's default binder
// cannot populate a map[string]string struct field, so these fields must be
// parsed out of the form manually. Returns nil when no matching fields exist so
// callers can distinguish "no form-supplied env vars" from an empty map and
// avoid clobbering values bound from a JSON body.
func parseFormEnvVars(values url.Values) map[string]string {
	var envVars map[string]string
	for key, vals := range values {
		if !strings.HasPrefix(key, envVarFormPrefix) || !strings.HasSuffix(key, "]") {
			continue
		}
		name := key[len(envVarFormPrefix) : len(key)-1]
		if name == "" {
			continue
		}
		value := ""
		if len(vals) > 0 {
			value = vals[0]
		}
		if envVars == nil {
			envVars = make(map[string]string)
		}
		envVars[name] = value
	}
	return envVars
}

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
	api.POST("/tests/:id/stop", h.stopTest)
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

	// Echo's binder cannot populate the EnvVars map[string]string field, so
	// parse the bracket-keyed env var form fields ourselves. Only overwrite
	// when the form actually carried env vars to preserve values bound from a
	// JSON request body.
	if values, err := c.FormValues(); err == nil {
		if envVars := parseFormEnvVars(values); envVars != nil {
			req.EnvVars = envVars
		}
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

func (h *Handler) stopTest(c *echo.Context) error {
	id := c.Param("id")
	if err := h.service.StopTest(c.Request().Context(), id); err != nil {
		if errors.Is(err, ErrTestFinished) {
			return Conflict(err.Error())
		}
		Logger().Error("Failed to stop test", slog.String("id", id), slog.Any("error", err))
		return InternalServerError()
	}
	return c.NoContent(http.StatusNoContent)
}

func (h *Handler) deleteTest(c *echo.Context) error {
	id := c.Param("id")
	if err := h.service.DeleteTest(c.Request().Context(), id); err != nil {
		Logger().Error("Failed to delete test", slog.String("id", id), slog.Any("error", err))
		return InternalServerError()
	}
	return c.NoContent(http.StatusNoContent)
}
