package internal

import (
	"bytes"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/url"
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
