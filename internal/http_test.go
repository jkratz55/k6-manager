package internal

import (
	"bytes"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/labstack/echo/v5"
)

func TestParseFormEnvVars(t *testing.T) {
	tests := []struct {
		name  string
		input url.Values
		want  map[string]string
	}{
		{
			name: "extracts bracket-keyed env vars",
			input: url.Values{
				"envVars[FOO]":      {"bar"},
				"envVars[BASE_URL]": {"https://example.com"},
				"name":              {"my-test"},
				"parallelism":       {"2"},
			},
			want: map[string]string{
				"FOO":      "bar",
				"BASE_URL": "https://example.com",
			},
		},
		{
			name: "keeps empty values",
			input: url.Values{
				"envVars[EMPTY]": {""},
			},
			want: map[string]string{"EMPTY": ""},
		},
		{
			name: "ignores empty key name",
			input: url.Values{
				"envVars[]": {"orphan"},
			},
			want: nil,
		},
		{
			name: "ignores malformed keys without closing bracket",
			input: url.Values{
				"envVars[FOO": {"bar"},
			},
			want: nil,
		},
		{
			name:  "returns nil when no env vars present",
			input: url.Values{"name": {"my-test"}},
			want:  nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseFormEnvVars(tt.input)
			if len(got) != len(tt.want) {
				t.Fatalf("expected %d entries, got %d: %v", len(tt.want), len(got), got)
			}
			for k, v := range tt.want {
				if got[k] != v {
					t.Errorf("key %s: expected %q, got %q", k, v, got[k])
				}
			}
		})
	}
}

// TestCreateTestRequestBindingEnvVars is a regression test proving that env vars
// submitted via a multipart form (as the UI sends them) end up on the request.
// It mirrors the binding sequence the createTest handler performs: Echo's
// c.Bind followed by manual parsing of the bracket-keyed env var fields.
func TestCreateTestRequestBindingEnvVars(t *testing.T) {
	body, contentType := multipartBody(t, map[string]string{
		"name":           "my-test",
		"parallelism":    "3",
		"runnerImage":    "grafana/k6:latest",
		"envVars[FOO]":   "bar",
		"envVars[TOKEN]": "secret",
	})

	req := httptest.NewRequest(http.MethodPost, "/api/tests", body)
	req.Header.Set(echo.HeaderContentType, contentType)
	c := echo.New().NewContext(req, httptest.NewRecorder())

	var got CreateTestRequest
	if err := c.Bind(&got); err != nil {
		t.Fatalf("bind: %v", err)
	}

	// Echo's binder alone must not have populated the map field...
	if got.EnvVars != nil {
		t.Fatalf("expected Echo binder to leave EnvVars nil, got %v", got.EnvVars)
	}

	// ...but the handler's manual parsing must recover them.
	values, err := c.FormValues()
	if err != nil {
		t.Fatalf("form values: %v", err)
	}
	if envVars := parseFormEnvVars(values); envVars != nil {
		got.EnvVars = envVars
	}

	// Scalar fields should still bind normally via Echo.
	if got.Name != "my-test" {
		t.Errorf("expected name my-test, got %s", got.Name)
	}
	if got.Parallelism != 3 {
		t.Errorf("expected parallelism 3, got %d", got.Parallelism)
	}

	want := map[string]string{"FOO": "bar", "TOKEN": "secret"}
	if len(got.EnvVars) != len(want) {
		t.Fatalf("expected %d env vars, got %d: %v", len(want), len(got.EnvVars), got.EnvVars)
	}
	for k, v := range want {
		if got.EnvVars[k] != v {
			t.Errorf("env %s: expected %q, got %q", k, v, got.EnvVars[k])
		}
	}
}

// multipartBody builds a multipart/form-data body from the given fields plus a
// dummy script file (required by the create request).
func multipartBody(t *testing.T, fields map[string]string) (*bytes.Buffer, string) {
	t.Helper()
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	for k, v := range fields {
		if err := w.WriteField(k, v); err != nil {
			t.Fatalf("write field %s: %v", k, err)
		}
	}
	fw, err := w.CreateFormFile("script", "script.js")
	if err != nil {
		t.Fatalf("create form file: %v", err)
	}
	if _, err := fw.Write([]byte("export default function () {}")); err != nil {
		t.Fatalf("write script: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("close writer: %v", err)
	}
	return &buf, w.FormDataContentType()
}

func TestParseFormSecretEnvVars(t *testing.T) {
	got := parseFormSecretEnvVars(url.Values{
		"secretEnvVars[TOKEN]":  {"api-creds/token"},
		"secretEnvVars[BROKEN]": {"no-key"},
		"envVars[NOT_A_SECRET]": {"x"},
	})
	want := map[string]SecretKeyRef{
		"TOKEN":  {Name: "api-creds", Key: "token"},
		"BROKEN": {Name: "no-key", Key: ""},
	}
	if len(got) != len(want) {
		t.Fatalf("expected %d entries, got %v", len(want), got)
	}
	for k, v := range want {
		if got[k] != v {
			t.Errorf("%s: expected %+v, got %+v", k, v, got[k])
		}
	}
}

// newTestServer wires the real routes, validator and error handler around a
// K6Service backed by fake Kubernetes clients.
func newTestServer(t *testing.T) *echo.Echo {
	t.Helper()
	svc, _, _ := newTestService(t, nil, testRunObject("done-12345678", "finished"))
	e := echo.New()
	e.Validator = &Validator{}
	e.HTTPErrorHandler = ProblemDetailsErrorHandler
	NewHandler(svc, 5).RegisterRoutes(e)
	return e
}

func serve(e *echo.Echo, req *http.Request) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	return rec
}

func TestHandlerStatusCodes(t *testing.T) {
	e := newTestServer(t)

	tests := []struct {
		name   string
		method string
		path   string
		want   int
	}{
		{"get missing test", http.MethodGet, "/api/tests/missing", http.StatusNotFound},
		{"stop missing test", http.MethodPost, "/api/tests/missing/stop", http.StatusNotFound},
		{"rerun missing test", http.MethodPost, "/api/tests/missing/rerun", http.StatusNotFound},
		{"rerun test without script", http.MethodPost, "/api/tests/done-12345678/rerun", http.StatusConflict},
		{"stop finished test", http.MethodPost, "/api/tests/done-12345678/stop", http.StatusConflict},
		{"delete missing test is idempotent", http.MethodDelete, "/api/tests/missing", http.StatusNoContent},
		{"unknown route", http.MethodGet, "/api/nope", http.StatusNotFound},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := serve(e, httptest.NewRequest(tt.method, tt.path, nil))
			if rec.Code != tt.want {
				t.Errorf("expected %d, got %d: %s", tt.want, rec.Code, rec.Body.String())
			}
		})
	}
}

func TestCreateTestValidation(t *testing.T) {
	e := newTestServer(t)

	t.Run("rejects parallelism above the configured maximum", func(t *testing.T) {
		body, contentType := multipartBody(t, map[string]string{"name": "big", "parallelism": "6"})
		req := httptest.NewRequest(http.MethodPost, "/api/tests", body)
		req.Header.Set(echo.HeaderContentType, contentType)
		rec := serve(e, req)
		if rec.Code != http.StatusUnprocessableEntity || !bytes.Contains(rec.Body.Bytes(), []byte(`"parallelism"`)) {
			t.Errorf("expected 422 with a parallelism error, got %d: %s", rec.Code, rec.Body.String())
		}
	})

	t.Run("rejects malformed secret references", func(t *testing.T) {
		body, contentType := multipartBody(t, map[string]string{
			"name": "s", "parallelism": "1", "secretEnvVars[TOKEN]": "Not_A_Valid_Name/key",
		})
		req := httptest.NewRequest(http.MethodPost, "/api/tests", body)
		req.Header.Set(echo.HeaderContentType, contentType)
		rec := serve(e, req)
		if rec.Code != http.StatusUnprocessableEntity || !bytes.Contains(rec.Body.Bytes(), []byte(`"secretEnvVars"`)) {
			t.Errorf("expected 422 with a secretEnvVars error, got %d: %s", rec.Code, rec.Body.String())
		}
	})

	t.Run("rejects oversized bodies", func(t *testing.T) {
		body, contentType := multipartBody(t, map[string]string{
			"name": "huge", "parallelism": "1", "args": strings.Repeat("x", maxRequestBytes),
		})
		req := httptest.NewRequest(http.MethodPost, "/api/tests", body)
		req.Header.Set(echo.HeaderContentType, contentType)
		rec := serve(e, req)
		if rec.Code != http.StatusRequestEntityTooLarge {
			t.Errorf("expected 413, got %d: %s", rec.Code, rec.Body.String())
		}
	})

	t.Run("creates a valid test", func(t *testing.T) {
		body, contentType := multipartBody(t, map[string]string{
			"name": "ok", "parallelism": "2", "secretEnvVars[TOKEN]": "api-creds/token",
		})
		req := httptest.NewRequest(http.MethodPost, "/api/tests", body)
		req.Header.Set(echo.HeaderContentType, contentType)
		rec := serve(e, req)
		if rec.Code != http.StatusCreated {
			t.Errorf("expected 201, got %d: %s", rec.Code, rec.Body.String())
		}
	})
}

func TestScriptSizeValidation(t *testing.T) {
	req := CreateTestRequest{
		Name:        "big",
		Parallelism: 1,
		Script:      &multipart.FileHeader{Filename: "script.js", Size: maxScriptBytes + 1},
	}
	errs := MapValidationErrors(req.Validate())
	if len(errs["script"]) == 0 {
		t.Fatalf("expected a script size error, got %v", errs)
	}
}
