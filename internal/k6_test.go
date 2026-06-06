package internal

import (
	"testing"
)

func TestBuildTestRun(t *testing.T) {
	req := CreateTestRequest{
		Name:        "test-run",
		Parallelism: 2,
		RunnerImage: "my-runner:latest",
		Args:        "--vus 10",
	}
	name := "test-run-123"
	namespace := "k6"
	defaultRunnerImage := "docker.io/grafana/k6:latest"
	defaultStarterImage := "my-artifactory/k6-operator:latest-starter"

	tr := buildTestRun(req, name, namespace, defaultRunnerImage, defaultStarterImage)

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
}

func TestBuildTestRunNoStarter(t *testing.T) {
	req := CreateTestRequest{
		Name:        "test-run",
		Parallelism: 2,
	}
	name := "test-run-123"
	namespace := "k6"
	defaultRunnerImage := "docker.io/grafana/k6:latest"
	defaultStarterImage := ""

	tr := buildTestRun(req, name, namespace, defaultRunnerImage, defaultStarterImage)

	spec := tr.Object["spec"].(map[string]any)

	if _, ok := spec["starter"]; ok {
		t.Error("expected no starter field in spec when defaultStarterImage is empty")
	}
}
