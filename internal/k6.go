package internal

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"maps"
	"slices"
	"strings"
	"time"

	"github.com/google/uuid"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
)

var (
	TestRunGVR = schema.GroupVersionResource{
		Group:    "k6.io",
		Version:  "v1alpha1",
		Resource: "testruns",
	}

	ErrTestFinished      = errors.New("test is already finished")
	ErrScriptUnavailable = errors.New("script content is not available")
)

const (
	scriptFileName = "test.js"
	managedByValue = "k6-manager"

	// stoppedAtAnnotation records when a test was stopped through k6-manager,
	// so a user-initiated stop can be told apart from a natural finish.
	stoppedAtAnnotation = "k6-manager/stopped-at"

	// k6-operator labels every Job/Pod it creates with k6_cr=<TestRun name>,
	// and runner Jobs additionally with runner=true.
	k6CRLabel = "k6_cr"

	// Kubernetes object names are capped at 63 characters for some of the
	// resources k6-operator derives from the TestRun name (e.g.
	// "<name>-initializer"), so keep the user-supplied part short.
	maxBaseNameLen = 40

	resultPassed = "passed"
	resultFailed = "failed"
)

type K6Service struct {
	client        kubernetes.Interface
	dynamicClient dynamic.Interface
	conf          AppConfig
}

func NewK6Service(
	client kubernetes.Interface,
	dynamicClient dynamic.Interface,
	config AppConfig) *K6Service {
	return &K6Service{
		client:        client,
		dynamicClient: dynamicClient,
		conf:          config,
	}
}

// testSpec is everything needed to launch a test run.
type testSpec struct {
	nameBase      string
	parallelism   int
	runnerImage   string
	args          string
	envVars       map[string]string
	secretEnvVars map[string]SecretKeyRef
	script        string
}

func (k *K6Service) CreateTest(ctx context.Context, req CreateTestRequest) (string, error) {
	file, err := req.Script.Open()
	if err != nil {
		return "", fmt.Errorf("read script: %w", err)
	}
	defer file.Close()
	script, err := io.ReadAll(file)
	if err != nil {
		return "", fmt.Errorf("read script: %w", err)
	}

	return k.createTestInternal(ctx, testSpec{
		nameBase:      req.Name,
		parallelism:   req.Parallelism,
		runnerImage:   req.RunnerImage,
		args:          req.Args,
		envVars:       req.EnvVars,
		secretEnvVars: req.SecretEnvVars,
		script:        string(script),
	})
}

func (k *K6Service) ReRunTest(ctx context.Context, id string) (string, error) {
	test, err := k.GetTest(ctx, id)
	if err != nil {
		return "", fmt.Errorf("get test %s: %w", id, err)
	}

	// Remove the unique ID suffix from the name to generate a new one
	// Names are like "mytest-8charid"
	baseName := test.Name
	if len(baseName) > 9 && baseName[len(baseName)-9] == '-' {
		baseName = baseName[:len(baseName)-9]
	}

	if test.ScriptContent == "" {
		return "", fmt.Errorf("test %s: %w", id, ErrScriptUnavailable)
	}

	return k.createTestInternal(ctx, testSpec{
		nameBase:      baseName,
		parallelism:   test.Parallelism,
		runnerImage:   test.RunnerImage,
		args:          test.Args,
		envVars:       test.EnvVars,
		secretEnvVars: test.SecretEnvVars,
		script:        test.ScriptContent,
	})
}

func (k *K6Service) createTestInternal(ctx context.Context, spec testSpec) (string, error) {
	name := generateName(spec.nameBase)
	namespace := k.conf.Namespace

	cm := buildConfigMap(name, namespace, spec.script)
	if _, err := k.client.CoreV1().
		ConfigMaps(namespace).
		Create(ctx, cm, metav1.CreateOptions{}); err != nil {
		return "", fmt.Errorf("create configmap %s/%s: %w", namespace, name, err)
	}

	Logger().Info(fmt.Sprintf("Created configmap %s/%s", namespace, name))

	tr := buildTestRun(name, namespace, spec, k.conf.DefaultRunnerImage, k.conf.DefaultStarterImage)
	created, err := k.dynamicClient.
		Resource(TestRunGVR).
		Namespace(namespace).
		Create(ctx, tr, metav1.CreateOptions{})
	if err != nil {
		// Don't leave the script ConfigMap behind without a TestRun. Use a
		// context that survives the request being cancelled.
		if delErr := k.client.CoreV1().
			ConfigMaps(namespace).
			Delete(context.WithoutCancel(ctx), name, metav1.DeleteOptions{}); delErr != nil && !apierrors.IsNotFound(delErr) {
			Logger().Error("Failed to roll back configmap after testrun creation failed",
				slog.String("configmap", name),
				slog.Any("error", delErr))
		}
		return "", fmt.Errorf("create testrun %s/%s: %w", namespace, name, err)
	}

	Logger().Info(fmt.Sprintf("Created testrun %s/%s", namespace, name))

	// Make the TestRun own its ConfigMap so Kubernetes garbage collects the
	// script if the TestRun is deleted outside of k6-manager. Failing to set
	// this is not fatal; DeleteTest and the cleanup worker remove both anyway.
	if err := k.setConfigMapOwner(ctx, namespace, name, created); err != nil {
		Logger().Warn("Failed to set owner reference on configmap",
			slog.String("configmap", name),
			slog.Any("error", err))
	}

	return name, nil
}

func (k *K6Service) setConfigMapOwner(ctx context.Context, namespace, name string, owner *unstructured.Unstructured) error {
	patch, err := json.Marshal(map[string]any{
		"metadata": map[string]any{
			"ownerReferences": []metav1.OwnerReference{{
				APIVersion: owner.GetAPIVersion(),
				Kind:       owner.GetKind(),
				Name:       owner.GetName(),
				UID:        owner.GetUID(),
			}},
		},
	})
	if err != nil {
		return err
	}
	_, err = k.client.CoreV1().
		ConfigMaps(namespace).
		Patch(ctx, name, types.MergePatchType, patch, metav1.PatchOptions{})
	return err
}

func (k *K6Service) GetTests(ctx context.Context) ([]TestStatus, error) {
	namespace := k.conf.Namespace

	list, err := k.dynamicClient.
		Resource(TestRunGVR).
		Namespace(namespace).
		List(ctx, metav1.ListOptions{
			LabelSelector: fmt.Sprintf("%s=%s", "app.kubernetes.io/managed-by", managedByValue),
		})
	if err != nil {
		return nil, fmt.Errorf("list testruns: %w", err)
	}

	results := k.runnerResults(ctx, "")

	statuses := make([]TestStatus, 0, len(list.Items))
	for _, item := range list.Items {
		status := mapToTestStatus(&item)
		applyResult(status, results)
		statuses = append(statuses, *status)
	}

	return statuses, nil
}

func (k *K6Service) GetTest(ctx context.Context, id string) (*TestStatus, error) {
	namespace := k.conf.Namespace

	item, err := k.dynamicClient.
		Resource(TestRunGVR).
		Namespace(namespace).
		Get(ctx, id, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("get testrun %s/%s: %w", namespace, id, err)
	}

	status := mapToTestStatus(item)
	applyResult(status, k.runnerResults(ctx, id))

	if status.ConfigMap != "" {
		cm, err := k.client.CoreV1().ConfigMaps(namespace).Get(ctx, status.ConfigMap, metav1.GetOptions{})
		if err == nil {
			status.ScriptContent = cm.Data[status.Script]
		} else {
			Logger().Warn("Failed to fetch script from configmap",
				slog.String("configmap", status.ConfigMap),
				slog.Any("error", err))
		}
	}

	return status, nil
}

// runnerResults derives a pass/fail result per TestRun from its runner Jobs.
// k6-operator only reports that a run "finished", but a runner whose k6
// process exits non-zero (e.g. failed thresholds) leaves a failed Job behind.
// When id is empty, results for every TestRun in the namespace are returned.
// Errors are logged rather than returned: the result is best-effort and must
// not break listing tests.
func (k *K6Service) runnerResults(ctx context.Context, id string) map[string]string {
	selector := "app=k6,runner=true"
	if id != "" {
		selector += fmt.Sprintf(",%s=%s", k6CRLabel, id)
	}

	jobs, err := k.client.BatchV1().
		Jobs(k.conf.Namespace).
		List(ctx, metav1.ListOptions{LabelSelector: selector})
	if err != nil {
		Logger().Warn("Failed to list runner jobs; test results will be unavailable",
			slog.Any("error", err))
		return nil
	}

	return summarizeRunnerJobs(jobs.Items)
}

func summarizeRunnerJobs(jobs []batchv1.Job) map[string]string {
	results := make(map[string]string)
	pending := make(map[string]bool)
	for _, job := range jobs {
		name := job.Labels[k6CRLabel]
		if name == "" {
			continue
		}
		switch {
		case job.Status.Failed > 0:
			results[name] = resultFailed
		case job.Status.Succeeded == 0:
			pending[name] = true
		case results[name] == "":
			results[name] = resultPassed
		}
	}
	// A run only passed once every runner succeeded; one failure is enough to
	// call it failed.
	for name := range pending {
		if results[name] != resultFailed {
			delete(results, name)
		}
	}
	return results
}

func applyResult(status *TestStatus, results map[string]string) {
	// A user-initiated stop kills the runners, which shows up as failed Jobs;
	// that isn't a meaningful test result.
	if !status.Finished || status.StoppedAt != "" {
		return
	}
	status.Result = results[status.ID]
}

func (k *K6Service) StopTest(ctx context.Context, id string) error {
	namespace := k.conf.Namespace

	// Check if the test is already finished before trying to stop it
	item, err := k.dynamicClient.
		Resource(TestRunGVR).
		Namespace(namespace).
		Get(ctx, id, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("get testrun %s/%s: %w", namespace, id, err)
	}

	if phase := extractPhase(item); isTestFinished(phase) {
		return fmt.Errorf("%w: phase is %s", ErrTestFinished, phase)
	}

	// Delete all pods k6-operator created for this TestRun. Runner Jobs have
	// backoffLimit 0, so they fail instead of recreating the pods, and the
	// operator then moves the TestRun to "stopped" and "finished" on its own.
	// Deleting the Jobs instead would leave the TestRun stuck in "started"
	// because the operator waits for the runner Jobs to complete.
	labelSelector := fmt.Sprintf("%s=%s", k6CRLabel, id)
	err = k.client.CoreV1().Pods(namespace).DeleteCollection(ctx, metav1.DeleteOptions{}, metav1.ListOptions{
		LabelSelector: labelSelector,
	})
	if err != nil {
		return fmt.Errorf("stop test (delete pods) %s/%s: %w", namespace, id, err)
	}

	Logger().Info("Stopped test by deleting pods",
		slog.String("id", id),
		slog.String("labelSelector", labelSelector))

	patch, err := json.Marshal(map[string]any{
		"metadata": map[string]any{
			"annotations": map[string]string{
				stoppedAtAnnotation: time.Now().UTC().Format(time.RFC3339),
			},
		},
	})
	if err != nil {
		return fmt.Errorf("build stop annotation patch: %w", err)
	}
	if _, err := k.dynamicClient.
		Resource(TestRunGVR).
		Namespace(namespace).
		Patch(ctx, id, types.MergePatchType, patch, metav1.PatchOptions{}); err != nil {
		// The test has been stopped; only the bookkeeping failed.
		Logger().Warn("Failed to annotate stopped testrun",
			slog.String("id", id),
			slog.Any("error", err))
	}

	return nil
}

// DeleteTest removes the TestRun and its script ConfigMap. It is idempotent:
// resources that are already gone are not treated as errors, so a test whose
// ConfigMap was removed out-of-band can still be deleted.
func (k *K6Service) DeleteTest(ctx context.Context, id string) error {
	namespace := k.conf.Namespace

	if err := k.dynamicClient.
		Resource(TestRunGVR).
		Namespace(namespace).
		Delete(ctx, id, metav1.DeleteOptions{}); err != nil && !apierrors.IsNotFound(err) {
		return fmt.Errorf("delete testrun %s/%s: %w", namespace, id, err)
	}

	if err := k.client.CoreV1().
		ConfigMaps(namespace).
		Delete(ctx, id, metav1.DeleteOptions{}); err != nil && !apierrors.IsNotFound(err) {
		return fmt.Errorf("delete configmap %s/%s: %w", namespace, id, err)
	}

	return nil
}

func (k *K6Service) CleanupTests(ctx context.Context, olderThan time.Duration) error {
	namespace := k.conf.Namespace
	logger := Logger()

	list, err := k.dynamicClient.
		Resource(TestRunGVR).
		Namespace(namespace).
		List(ctx, metav1.ListOptions{
			LabelSelector: fmt.Sprintf("%s=%s", "app.kubernetes.io/managed-by", managedByValue),
		})
	if err != nil {
		return fmt.Errorf("list testruns for cleanup: %w", err)
	}

	cutoff := time.Now().Add(-olderThan)
	for _, item := range list.Items {
		created := item.GetCreationTimestamp().Time
		if created.Before(cutoff) {
			id := item.GetName()
			logger.Info("Cleaning up old test",
				slog.String("id", id),
				slog.Time("created", created))
			if err := k.DeleteTest(ctx, id); err != nil {
				logger.Error("Failed to delete old test during cleanup",
					slog.String("id", id),
					slog.Any("error", err))
				continue
			}
		}
	}

	return nil
}

func mapToTestStatus(item *unstructured.Unstructured) *TestStatus {
	parallelism, _, _ := unstructured.NestedInt64(item.Object, "spec", "parallelism")
	configMap, _, _ := unstructured.NestedString(item.Object, "spec", "script", "configMap", "name")
	scriptFile, _, _ := unstructured.NestedString(item.Object, "spec", "script", "configMap", "file")
	runnerImage, _, _ := unstructured.NestedString(item.Object, "spec", "runner", "image")
	args, _, _ := unstructured.NestedString(item.Object, "spec", "arguments")

	envVars := make(map[string]string)
	secretEnvVars := make(map[string]SecretKeyRef)
	if env, ok, _ := unstructured.NestedSlice(item.Object, "spec", "runner", "env"); ok {
		for _, e := range env {
			m, ok := e.(map[string]any)
			if !ok {
				continue
			}
			name, _, _ := unstructured.NestedString(m, "name")
			if name == "" {
				continue
			}
			if secretName, ok, _ := unstructured.NestedString(m, "valueFrom", "secretKeyRef", "name"); ok {
				key, _, _ := unstructured.NestedString(m, "valueFrom", "secretKeyRef", "key")
				secretEnvVars[name] = SecretKeyRef{Name: secretName, Key: key}
				continue
			}
			value, _, _ := unstructured.NestedString(m, "value")
			envVars[name] = value
		}
	}

	phase := extractPhase(item)
	return &TestStatus{
		ID:            item.GetName(),
		Name:          item.GetName(),
		Namespace:     item.GetNamespace(),
		Phase:         phase,
		Finished:      isTestFinished(phase),
		Parallelism:   int(parallelism),
		StartedAt:     item.GetCreationTimestamp().UTC().Format(time.RFC3339),
		FinishedAt:    extractFinishedAt(item),
		StoppedAt:     item.GetAnnotations()[stoppedAtAnnotation],
		ConfigMap:     configMap,
		Script:        scriptFile,
		RunnerImage:   runnerImage,
		Args:          args,
		EnvVars:       envVars,
		SecretEnvVars: secretEnvVars,
	}
}

// generateName builds a unique, valid Kubernetes object name from a
// user-supplied test name. The result always starts with a letter, because
// k6-operator derives Service names (DNS-1035 labels) from it.
func generateName(base string) string {
	id := uuid.New().String()[:8]
	name := sanitizeName(base)
	if name == "" {
		name = "k6-run"
	}
	return fmt.Sprintf("%s-%s", name, id)
}

// sanitizeName lowercases s, replaces runs of characters that aren't valid in
// a DNS label with a single '-', and ensures the result starts with a letter.
// It returns "" when nothing usable remains.
func sanitizeName(s string) string {
	s = strings.ToLower(s)
	var b strings.Builder
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
		} else if !strings.HasSuffix(b.String(), "-") {
			b.WriteRune('-')
		}
	}
	name := strings.Trim(b.String(), "-")
	if name == "" {
		return ""
	}
	if name[0] < 'a' || name[0] > 'z' {
		name = "k6-" + name
	}
	if len(name) > maxBaseNameLen {
		name = strings.TrimRight(name[:maxBaseNameLen], "-")
	}
	return name
}

func buildConfigMap(name, namespace, script string) *corev1.ConfigMap {
	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels: map[string]string{
				"app.kubernetes.io/managed-by": managedByValue,
				"k6-manager/testrun-name":      name,
			},
		},
		Data: map[string]string{
			scriptFileName: script,
		},
	}
}

func buildTestRun(name, namespace string, spec testSpec, defaultImage, defaultStarterImage string) *unstructured.Unstructured {
	parallelism := spec.parallelism
	if parallelism <= 0 {
		parallelism = 1
	}

	runnerImage := spec.runnerImage
	if runnerImage == "" {
		runnerImage = defaultImage
	}

	// Build env var list, sorted so the generated spec is deterministic.
	envVars := []any{}
	for _, k := range slices.Sorted(maps.Keys(spec.envVars)) {
		envVars = append(envVars, map[string]any{
			"name":  k,
			"value": spec.envVars[k],
		})
	}
	for _, k := range slices.Sorted(maps.Keys(spec.secretEnvVars)) {
		ref := spec.secretEnvVars[k]
		envVars = append(envVars, map[string]any{
			"name": k,
			"valueFrom": map[string]any{
				"secretKeyRef": map[string]any{
					"name": ref.Name,
					"key":  ref.Key,
				},
			},
		})
	}

	trSpec := map[string]any{
		"parallelism": int64(parallelism),
		"script": map[string]any{
			"configMap": map[string]any{
				"name": name,
				"file": scriptFileName,
			},
		},
		"runner": map[string]any{
			"image": runnerImage,
			"env":   envVars,
		},
	}

	if defaultStarterImage != "" {
		trSpec["starter"] = map[string]any{
			"image": defaultStarterImage,
		}
	}

	if spec.args != "" {
		trSpec["arguments"] = spec.args
	}

	return &unstructured.Unstructured{
		Object: map[string]any{
			"apiVersion": fmt.Sprintf("%s/%s", TestRunGVR.Group, TestRunGVR.Version),
			"kind":       "TestRun",
			"metadata": map[string]any{
				"name":      name,
				"namespace": namespace,
				"labels": map[string]any{
					"app.kubernetes.io/managed-by": managedByValue,
					"k6-manager/testrun-name":      name,
				},
			},
			"spec": trSpec,
		},
	}
}

// isTestFinished reports whether the runners for a TestRun are done. These
// are k6-operator stages: "stopped" is the brief stage after all runner Jobs
// complete and before "finished".
func isTestFinished(phase string) bool {
	switch phase {
	case "stopped", "finished", "error":
		return true
	}
	return false
}

func extractPhase(obj *unstructured.Unstructured) string {
	phase, _, _ := unstructured.NestedString(obj.Object, "status", "stage")
	if phase == "" {
		phase = "created"
	}
	return strings.ToLower(phase)
}

func extractFinishedAt(obj *unstructured.Unstructured) string {
	phase := extractPhase(obj)
	if !isTestFinished(phase) {
		return ""
	}

	conditions, _, _ := unstructured.NestedSlice(obj.Object, "status", "conditions")
	var latest string
	for _, c := range conditions {
		cond, ok := c.(map[string]any)
		if !ok {
			continue
		}
		t, _, _ := unstructured.NestedString(cond, "lastTransitionTime")
		if t > latest {
			latest = t
		}
	}

	if latest == "" {
		return ""
	}
	parsed, err := time.Parse(time.RFC3339, latest)
	if err != nil {
		return latest
	}
	return parsed.Format(time.RFC3339)
}
