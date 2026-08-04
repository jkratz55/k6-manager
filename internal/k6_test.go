package internal

import (
	"testing"
)

func TestBuildTestRun(t *testing.T) {
	name := "test-run-123"
	namespace := "k6"
	defaultRunnerImage := "docker.io/grafana/k6:latest"
	defaultStarterImage := "my-artifactory/k6-operator:latest-starter"
	envVars := map[string]string{
		"BASE_URL": "https://example.com",
		"TOKEN":    "secret",
	}

	tr := buildTestRun(name, namespace, 2, "my-runner:latest", "--vus 10", envVars, defaultRunnerImage, defaultStarterImage)

	if tr.GetName() != name {
		t.Errorf("expected name %s, got %s", name, tr.GetName())
	}

	spec := tr.Object["spec"].(map[string]any)

	starter, ok := spec["starter"].(map[string]any)
	if !ok {
		t.Fatal("expected starter field in spec")
	}

	if starter["image"] != defaultStarterImage {
		t.Errorf("expected starter image %s, got %s", defaultStarterImage, starter["image"])
	}

	runner := spec["runner"].(map[string]any)
	if runner["image"] != "my-runner:latest" {
		t.Errorf("expected runner image %s, got %s", "my-runner:latest", runner["image"])
	}

	gotEnv := envSliceToMap(t, runner["env"])
	if len(gotEnv) != len(envVars) {
		t.Fatalf("expected %d env vars, got %d: %v", len(envVars), len(gotEnv), gotEnv)
	}
	for k, v := range envVars {
		if gotEnv[k] != v {
			t.Errorf("expected env %s=%s, got %s", k, v, gotEnv[k])
		}
	}
}

func TestBuildTestRunNoStarter(t *testing.T) {
	name := "test-run-123"
	namespace := "k6"
	defaultRunnerImage := "docker.io/grafana/k6:latest"
	defaultStarterImage := ""

	tr := buildTestRun(name, namespace, 2, "", "", nil, defaultRunnerImage, defaultStarterImage)

	spec := tr.Object["spec"].(map[string]any)

	if _, ok := spec["starter"]; ok {
		t.Error("expected no starter field in spec when defaultStarterImage is empty")
	}

	runner := spec["runner"].(map[string]any)
	if runner["image"] != defaultRunnerImage {
		t.Errorf("expected runner image to default to %s, got %s", defaultRunnerImage, runner["image"])
	}
}

// envSliceToMap converts the spec.runner.env slice produced by buildTestRun
// back into a name->value map for easy assertion.
func envSliceToMap(t *testing.T, raw any) map[string]string {
	t.Helper()
	envSlice, ok := raw.([]any)
	if !ok {
		t.Fatalf("expected runner env to be []any, got %T", raw)
	}
	out := make(map[string]string, len(envSlice))
	for _, e := range envSlice {
		m, ok := e.(map[string]any)
		if !ok {
			t.Fatalf("expected env entry to be map[string]any, got %T", e)
		}
		out[m["name"].(string)] = m["value"].(string)
	}
	return out
}
