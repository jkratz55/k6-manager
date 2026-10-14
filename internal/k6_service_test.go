package internal

import (
	"context"
	"errors"
	"regexp"
	"strings"
	"testing"

	batchv1 "k8s.io/api/batch/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	k8svalidation "k8s.io/apimachinery/pkg/util/validation"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	"k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"
)

const testNamespace = "k6"

func newTestService(t *testing.T, objects []runtime.Object, testRuns ...runtime.Object) (*K6Service, *fake.Clientset, *dynamicfake.FakeDynamicClient) {
	t.Helper()
	client := fake.NewClientset(objects...)
	dyn := dynamicfake.NewSimpleDynamicClientWithCustomListKinds(runtime.NewScheme(),
		map[schema.GroupVersionResource]string{TestRunGVR: "TestRunList"}, testRuns...)
	svc := NewK6Service(client, dyn, AppConfig{
		Namespace:          testNamespace,
		DefaultRunnerImage: "grafana/k6:test",
	})
	return svc, client, dyn
}

func testRunObject(name, stage string) *unstructured.Unstructured {
	tr := buildTestRun(name, testNamespace, testSpec{parallelism: 1}, "grafana/k6:test", "")
	if stage != "" {
		_ = unstructured.SetNestedField(tr.Object, stage, "status", "stage")
	}
	return tr
}

func TestGenerateNameProducesValidNames(t *testing.T) {
	dns1035 := regexp.MustCompile(`^[a-z]([-a-z0-9]*[a-z0-9])?$`)

	tests := []struct {
		in         string
		wantPrefix string
	}{
		{"Homepage Load Test", "homepage-load-test-"},
		{"!!!", "k6-run-"},
		{"负载测试", "k6-run-"},
		{"", "k6-run-"},
		{"123 checkout", "k6-123-checkout-"},
		{"a  --  b", "a-b-"},
		{strings.Repeat("x", 39) + "-yyyy", strings.Repeat("x", 39) + "-"},
	}
	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			got := generateName(tt.in)
			if !strings.HasPrefix(got, tt.wantPrefix) {
				t.Errorf("generateName(%q) = %q, want prefix %q", tt.in, got, tt.wantPrefix)
			}
			if !dns1035.MatchString(got) {
				t.Errorf("generateName(%q) = %q is not a valid DNS-1035 label", tt.in, got)
			}
			// k6-operator appends suffixes such as "-initializer" and
			// "-service-N" to derive names that must stay within 63 chars.
			if errs := k8svalidation.IsDNS1035Label(got + "-initializer"); len(errs) > 0 {
				t.Errorf("derived name too long for %q: %v", got, errs)
			}
		})
	}
}

func TestDeleteTestToleratesMissingConfigMap(t *testing.T) {
	svc, _, dyn := newTestService(t, nil, testRunObject("orphan-12345678", "finished"))

	if err := svc.DeleteTest(context.Background(), "orphan-12345678"); err != nil {
		t.Fatalf("DeleteTest: %v", err)
	}

	_, err := dyn.Resource(TestRunGVR).Namespace(testNamespace).Get(context.Background(), "orphan-12345678", metav1.GetOptions{})
	if !apierrors.IsNotFound(err) {
		t.Fatalf("expected testrun to be deleted, got err=%v", err)
	}

	// Deleting again is a no-op rather than an error.
	if err := svc.DeleteTest(context.Background(), "orphan-12345678"); err != nil {
		t.Fatalf("second DeleteTest: %v", err)
	}
}

func TestCreateTestRollsBackConfigMapWhenTestRunFails(t *testing.T) {
	svc, client, dyn := newTestService(t, nil)
	dyn.PrependReactor("create", "testruns", func(k8stesting.Action) (bool, runtime.Object, error) {
		return true, nil, errors.New("boom")
	})

	if _, err := svc.createTestInternal(context.Background(), testSpec{nameBase: "rollback", parallelism: 1, script: "x"}); err == nil {
		t.Fatal("expected an error")
	}

	cms, err := client.CoreV1().ConfigMaps(testNamespace).List(context.Background(), metav1.ListOptions{})
	if err != nil {
		t.Fatalf("list configmaps: %v", err)
	}
	if len(cms.Items) != 0 {
		t.Fatalf("expected configmap to be rolled back, found %d", len(cms.Items))
	}
}

func TestCreateTestSetsConfigMapOwner(t *testing.T) {
	svc, client, _ := newTestService(t, nil)

	name, err := svc.createTestInternal(context.Background(), testSpec{nameBase: "owned", parallelism: 1, script: "x"})
	if err != nil {
		t.Fatalf("createTestInternal: %v", err)
	}

	cm, err := client.CoreV1().ConfigMaps(testNamespace).Get(context.Background(), name, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("get configmap: %v", err)
	}
	if len(cm.OwnerReferences) != 1 || cm.OwnerReferences[0].Kind != "TestRun" || cm.OwnerReferences[0].Name != name {
		t.Fatalf("expected TestRun owner reference, got %+v", cm.OwnerReferences)
	}
}

func TestStopTest(t *testing.T) {
	t.Run("deletes pods and records the stop", func(t *testing.T) {
		svc, client, dyn := newTestService(t, nil, testRunObject("running-12345678", "started"))

		if err := svc.StopTest(context.Background(), "running-12345678"); err != nil {
			t.Fatalf("StopTest: %v", err)
		}

		var deletedPods bool
		for _, a := range client.Actions() {
			if a.Matches("delete-collection", "pods") {
				deletedPods = true
			}
		}
		if !deletedPods {
			t.Error("expected pods to be deleted")
		}

		tr, err := dyn.Resource(TestRunGVR).Namespace(testNamespace).Get(context.Background(), "running-12345678", metav1.GetOptions{})
		if err != nil {
			t.Fatalf("get testrun: %v", err)
		}
		if mapToTestStatus(tr).StoppedAt == "" {
			t.Error("expected stoppedAt to be recorded")
		}
	})

	t.Run("rejects finished tests", func(t *testing.T) {
		for _, stage := range []string{"stopped", "finished", "error"} {
			svc, _, _ := newTestService(t, nil, testRunObject("done-12345678", stage))
			if err := svc.StopTest(context.Background(), "done-12345678"); !errors.Is(err, ErrTestFinished) {
				t.Errorf("stage %s: expected ErrTestFinished, got %v", stage, err)
			}
		}
	})

	t.Run("reports missing tests as not found", func(t *testing.T) {
		svc, _, _ := newTestService(t, nil)
		if err := svc.StopTest(context.Background(), "missing"); !apierrors.IsNotFound(err) {
			t.Errorf("expected not found, got %v", err)
		}
	})
}

func TestGetTestsDerivesResultFromRunnerJobs(t *testing.T) {
	job := func(testRun, suffix string, succeeded, failed int32) runtime.Object {
		return &batchv1.Job{
			ObjectMeta: metav1.ObjectMeta{
				Name:      testRun + "-" + suffix,
				Namespace: testNamespace,
				Labels:    map[string]string{"app": "k6", "runner": "true", k6CRLabel: testRun},
			},
			Status: batchv1.JobStatus{Succeeded: succeeded, Failed: failed},
		}
	}

	stopped := testRunObject("stopped-1", "finished")
	stopped.SetAnnotations(map[string]string{stoppedAtAnnotation: "2026-01-01T00:00:00Z"})

	svc, _, _ := newTestService(t,
		[]runtime.Object{
			job("passed-1", "1", 1, 0), job("passed-1", "2", 1, 0),
			job("failed-1", "1", 1, 0), job("failed-1", "2", 0, 1),
			job("running-1", "1", 0, 0),
			job("stopped-1", "1", 0, 1),
		},
		testRunObject("passed-1", "finished"),
		testRunObject("failed-1", "finished"),
		testRunObject("running-1", "started"),
		testRunObject("nojobs-1", "finished"),
		stopped,
	)

	tests, err := svc.GetTests(context.Background())
	if err != nil {
		t.Fatalf("GetTests: %v", err)
	}

	want := map[string]struct {
		finished bool
		result   string
	}{
		"passed-1":  {true, resultPassed},
		"failed-1":  {true, resultFailed},
		"running-1": {false, ""},
		"nojobs-1":  {true, ""},
		"stopped-1": {true, ""},
	}
	if len(tests) != len(want) {
		t.Fatalf("expected %d tests, got %d", len(want), len(tests))
	}
	for _, got := range tests {
		w := want[got.ID]
		if got.Finished != w.finished || got.Result != w.result {
			t.Errorf("%s: got finished=%v result=%q, want finished=%v result=%q",
				got.ID, got.Finished, got.Result, w.finished, w.result)
		}
	}
}

func TestSecretEnvVarsRoundTrip(t *testing.T) {
	spec := testSpec{
		parallelism:   1,
		envVars:       map[string]string{"BASE_URL": "https://example.com"},
		secretEnvVars: map[string]SecretKeyRef{"TOKEN": {Name: "api-creds", Key: "token"}},
	}
	status := mapToTestStatus(buildTestRun("secret-12345678", testNamespace, spec, "img", ""))

	if status.EnvVars["BASE_URL"] != "https://example.com" || len(status.EnvVars) != 1 {
		t.Errorf("unexpected env vars: %v", status.EnvVars)
	}
	if got := status.SecretEnvVars["TOKEN"]; got != (SecretKeyRef{Name: "api-creds", Key: "token"}) || len(status.SecretEnvVars) != 1 {
		t.Errorf("unexpected secret env vars: %v", status.SecretEnvVars)
	}
}

func TestReRunPreservesSpecAndReportsMissingScript(t *testing.T) {
	spec := testSpec{
		parallelism:   3,
		secretEnvVars: map[string]SecretKeyRef{"TOKEN": {Name: "api-creds", Key: "token"}},
	}
	tr := buildTestRun("orig-12345678", testNamespace, spec, "img", "")
	cm := buildConfigMap("orig-12345678", testNamespace, "export default function () {}")
	svc, _, dyn := newTestService(t, []runtime.Object{cm}, tr, testRunObject("noscript-12345678", "finished"))

	id, err := svc.ReRunTest(context.Background(), "orig-12345678")
	if err != nil {
		t.Fatalf("ReRunTest: %v", err)
	}
	if !strings.HasPrefix(id, "orig-") || id == "orig-12345678" {
		t.Errorf("unexpected new id %q", id)
	}
	created, err := dyn.Resource(TestRunGVR).Namespace(testNamespace).Get(context.Background(), id, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("get rerun testrun: %v", err)
	}
	status := mapToTestStatus(created)
	if status.Parallelism != 3 || status.SecretEnvVars["TOKEN"].Name != "api-creds" {
		t.Errorf("rerun did not preserve spec: %+v", status)
	}

	if _, err := svc.ReRunTest(context.Background(), "noscript-12345678"); !errors.Is(err, ErrScriptUnavailable) {
		t.Errorf("expected ErrScriptUnavailable, got %v", err)
	}
}
