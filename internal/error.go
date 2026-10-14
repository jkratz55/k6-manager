package internal

import (
	"errors"
	"net/http"

	"github.com/labstack/echo/v5"
)

func ProblemDetailsErrorHandler(c *echo.Context, err error) {
	if res, err := echo.UnwrapResponse(c.Response()); err == nil && res.Committed {
		return
	}

	var problem ErrorResponse

	if p, ok := errors.AsType[ErrorResponse](err); ok {
		problem = p
		problem.Instance = c.Request().RequestURI
	} else if he, ok := errors.AsType[*echo.HTTPError](err); ok {
		problem = ErrorResponse{
			Type:   "about:blank",
			Title:  http.StatusText(he.Code),
			Status: he.Code,
			Detail: he.Error(),
		}
	} else if code := echo.StatusCode(err); code != 0 {
		// Echo's sentinel errors (echo.ErrNotFound, echo.ErrMethodNotAllowed,
		// the body limit's 413, ...) aren't *echo.HTTPError but still carry
		// a status code.
		problem = ErrorResponse{
			Type:   "about:blank",
			Title:  http.StatusText(code),
			Status: code,
			Detail: err.Error(),
		}
	} else {
		problem = InternalServerError()
	}

	c.Response().Header().Set(echo.HeaderContentType, "application/problem+json; charset=UTF-8")
	if err := c.JSON(problem.Status, problem); err != nil {
		Logger().Error("failed to write problem details response", "error", err)
	}
}

func BadRequest() ErrorResponse {
	return ErrorResponse{
		Type:     "about:blank",
		Title:    "Bad Request",
		Status:   http.StatusBadRequest,
		Detail:   "Server could not understand the request as it is malformed or incomplete.",
		Instance: "",
		TraceID:  "",
		Errors:   nil,
	}
}

func UnprocessableEntity(err error) ErrorResponse {
	return ErrorResponse{
		Type:     "about:blank",
		Title:    "Unprocessable Entity",
		Status:   http.StatusUnprocessableEntity,
		Detail:   "Server understood the request but was unable to process it.",
		Instance: "",
		Errors:   MapValidationErrors(err),
	}
}

func InternalServerError() ErrorResponse {
	return ErrorResponse{
		Type:     "about:blank",
		Title:    "Internal Server Error",
		Status:   http.StatusInternalServerError,
		Detail:   "Server encountered an internal error while processing the request. Please try again later.",
		Instance: "",
		TraceID:  "",
		Errors:   nil,
	}
}

func Conflict(detail string) ErrorResponse {
	return ErrorResponse{
		Type:     "about:blank",
		Title:    "Conflict",
		Status:   http.StatusConflict,
		Detail:   detail,
		Instance: "",
		TraceID:  "",
		Errors:   nil,
	}
}

func NotFound(detail string) ErrorResponse {
	return ErrorResponse{
		Type:     "about:blank",
		Title:    "Not Found",
		Status:   http.StatusNotFound,
		Detail:   detail,
		Instance: "",
		TraceID:  "",
		Errors:   nil,
	}
}

func RequestEntityTooLarge(detail string) ErrorResponse {
	return ErrorResponse{
		Type:     "about:blank",
		Title:    "Request Entity Too Large",
		Status:   http.StatusRequestEntityTooLarge,
		Detail:   detail,
		Instance: "",
		TraceID:  "",
		Errors:   nil,
	}
}
