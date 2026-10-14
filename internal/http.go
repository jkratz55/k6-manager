package internal

import (
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"strings"

	validation "github.com/go-ozzo/ozzo-validation/v4"
	"github.com/labstack/echo/v5"
	"github.com/labstack/echo/v5/middleware"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
)

const (
	envVarFormPrefix       = "envVars["
	secretEnvVarFormPrefix = "secretEnvVars["

	// maxRequestBytes bounds the size of API request bodies. The largest
	// legitimate request is a test creation carrying a script of up to
	// maxScriptBytes plus a handful of small form fields.
	maxRequestBytes = 2 * 1024 * 1024
)

// parseFormEnvVars extracts environment variables submitted as bracket-keyed
// form fields (e.g. "envVars[FOO]=bar") into a map. Echo's default binder
// cannot populate a map[string]string struct field, so these fields must be
// parsed out of the form manually. Returns nil when no matching fields exist so
// callers can distinguish "no form-supplied env vars" from an empty map and
// avoid clobbering values bound from a JSON body.
func parseFormEnvVars(values url.Values) map[string]string {
	return parseBracketFields(values, envVarFormPrefix)
}

// parseFormSecretEnvVars extracts secret-backed environment variables
// submitted as "secretEnvVars[FOO]=<secret-name>/<key>". Neither secret names
// nor keys may contain '/', so the first '/' unambiguously separates them.
// Malformed values are kept with an empty key so validation reports them.
func parseFormSecretEnvVars(values url.Values) map[string]SecretKeyRef {
	fields := parseBracketFields(values, secretEnvVarFormPrefix)
	if fields == nil {
		return nil
	}
	refs := make(map[string]SecretKeyRef, len(fields))
	for name, value := range fields {
		secretName, key, _ := strings.Cut(value, "/")
		refs[name] = SecretKeyRef{Name: secretName, Key: key}
	}
	return refs
}

func parseBracketFields(values url.Values, prefix string) map[string]string {
	var fields map[string]string
	for key, vals := range values {
		if !strings.HasPrefix(key, prefix) || !strings.HasSuffix(key, "]") {
			continue
		}
		name := key[len(prefix) : len(key)-1]
		if name == "" {
			continue
		}
		value := ""
		if len(vals) > 0 {
			value = vals[0]
		}
		if fields == nil {
			fields = make(map[string]string)
		}
		fields[name] = value
	}
	return fields
}

type Handler struct {
	service        *K6Service
	maxParallelism int
}

func NewHandler(service *K6Service, maxParallelism int) *Handler {
	return &Handler{service: service, maxParallelism: maxParallelism}
}

func (h *Handler) RegisterRoutes(e *echo.Echo) {
	api := e.Group("/api", middleware.BodyLimit(maxRequestBytes))
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
		if apierrors.IsNotFound(err) {
			return testNotFound(id)
		}
		Logger().Error("Failed to get test", slog.String("id", id), slog.Any("error", err))
		return InternalServerError()
	}
	return c.JSON(http.StatusOK, res)
}

func (h *Handler) createTest(c *echo.Context) error {
	var req CreateTestRequest
	if err := c.Bind(&req); err != nil {
		if errors.Is(err, echo.ErrStatusRequestEntityTooLarge) {
			return RequestEntityTooLarge(fmt.Sprintf("Request body must be no larger than %d bytes.", maxRequestBytes))
		}
		return BadRequest()
	}

	// Echo's binder cannot populate the env var map fields, so parse the
	// bracket-keyed form fields ourselves. Only overwrite when the form
	// actually carried them to preserve values bound from a JSON request body.
	if values, err := c.FormValues(); err == nil {
		if envVars := parseFormEnvVars(values); envVars != nil {
			req.EnvVars = envVars
		}
		if secretEnvVars := parseFormSecretEnvVars(values); secretEnvVars != nil {
			req.SecretEnvVars = secretEnvVars
		}
	}

	if err := c.Validate(req); err != nil {
		return UnprocessableEntity(err)
	}
	if req.Parallelism > h.maxParallelism {
		return UnprocessableEntity(validation.Errors{
			"parallelism": fmt.Errorf("must be no greater than %d", h.maxParallelism),
		})
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
		switch {
		case apierrors.IsNotFound(err):
			return testNotFound(id)
		case errors.Is(err, ErrScriptUnavailable):
			return Conflict(fmt.Sprintf("The script for test %s is no longer available, so it cannot be re-run.", id))
		}
		Logger().Error("Failed to rerun test", slog.String("id", id), slog.Any("error", err))
		return InternalServerError()
	}

	c.Response().Header().Set(echo.HeaderLocation, fmt.Sprintf("/api/tests/%s", res))
	return c.JSON(http.StatusCreated, map[string]string{"id": res})
}

func (h *Handler) stopTest(c *echo.Context) error {
	id := c.Param("id")
	if err := h.service.StopTest(c.Request().Context(), id); err != nil {
		switch {
		case apierrors.IsNotFound(err):
			return testNotFound(id)
		case errors.Is(err, ErrTestFinished):
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

func testNotFound(id string) ErrorResponse {
	return NotFound(fmt.Sprintf("Test %s was not found.", id))
}
