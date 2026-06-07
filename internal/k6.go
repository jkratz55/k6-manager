package internal

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"strings"
	"time"

	"github.com/google/uuid"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
)

var TestRunGVR = schema.GroupVersionResource{
	Group:    "k6.io",
	Version:  "v1alpha1",
	Resource: "testruns",
}

const (
	scriptFileName = "test.js"
	managedByValue = "k6-manager"
)

type K6Service struct {
	client        *kubernetes.Clientset
	dynamicClient dynamic.Interface
	conf          AppConfig
}

func NewK6Service(
	client *kubernetes.Clientset,
	dynamicClient dynamic.Interface,
	config AppConfig) *K6Service {
	return &K6Service{
		client:        client,
		dynamicClient: dynamicClient,
		conf:          config,
	}
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

	return k.createTestInternal(ctx, req.Name, req.Parallelism, req.RunnerImage, req.Args, req.EnvVars, string(script))
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
		return "", fmt.Errorf("script content for test %s is not available", id)
	}

	return k.createTestInternal(ctx, baseName, test.Parallelism, test.RunnerImage, test.Args, test.EnvVars, test.ScriptContent)
}

func (k *K6Service) createTestInternal(ctx context.Context, nameBase string, parallelism int, runnerImage string, args string, envVars map[string]string, script string) (string, error) {
	name := generateName(nameBase)
	namespace := k.conf.Namespace

	cm := buildConfigMap(name, namespace, script)
	if _, err := k.client.CoreV1().
		ConfigMaps(namespace).
		Create(ctx, cm, metav1.CreateOptions{}); err != nil {
		return "", fmt.Errorf("create configmap %s/%s: %w", namespace, name, err)
	}

	Logger().Info(fmt.Sprintf("Created configmap %s/%s", namespace, name))

	tr := buildTestRun(name, namespace, parallelism, runnerImage, args, envVars, k.conf.DefaultRunnerImage, k.conf.DefaultStarterImage)
	_, err := k.dynamicClient.
		Resource(TestRunGVR).
		Namespace(namespace).
		Create(ctx, tr, metav1.CreateOptions{})
	if err != nil {
		return "", fmt.Errorf("create testrun %s/%s: %w", namespace, name, err)
	}

	Logger().Info(fmt.Sprintf("Created testrun %s/%s", namespace, name))

	return name, nil
}

func (k *K6Service) GetTests(ctx context.Context) (any, error) {
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

	results := make([]TestStatus, 0, len(list.Items))
	for _, item := range list.Items {
		results = append(results, *mapToTestStatus(&item))
	}

	return results, nil
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

func (k *K6Service) DeleteTest(ctx context.Context, id string) error {
	namespace := k.conf.Namespace

	if err := k.client.CoreV1().
		ConfigMaps(namespace).
		Delete(ctx, id, metav1.DeleteOptions{}); err != nil {
		return fmt.Errorf("delete configmap %s/%s: %w", namespace, id, err)
	}

	if err := k.dynamicClient.
		Resource(TestRunGVR).
		Namespace(namespace).
		Delete(ctx, id, metav1.DeleteOptions{}); err != nil {
		return fmt.Errorf("delete testrun %s/%s: %w", namespace, id, err)
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
	if env, ok, _ := unstructured.NestedSlice(item.Object, "spec", "runner", "env"); ok {
		for _, e := range env {
			if m, ok := e.(map[string]any); ok {
				name, _, _ := unstructured.NestedString(m, "name")
				value, _, _ := unstructured.NestedString(m, "value")
				if name != "" {
					envVars[name] = value
				}
			}
		}
	}

	return &TestStatus{
		ID:          item.GetName(),
		Name:        item.GetName(),
		Namespace:   item.GetNamespace(),
		Phase:       extractPhase(item),
		Parallelism: int(parallelism),
		StartedAt:   item.GetCreationTimestamp().UTC().Format(time.RFC3339),
		FinishedAt:  extractFinishedAt(item),
		ConfigMap:   configMap,
		Script:      scriptFile,
		RunnerImage: runnerImage,
		Args:        args,
		EnvVars:     envVars,
	}
}

func generateName(base string) string {
	id := uuid.New().String()[:8]
	if base != "" {
		return fmt.Sprintf("%s-%s", sanitizeName(base), id)
	}
	return fmt.Sprintf("k6-run-%s", id)
}

func sanitizeName(s string) string {
	s = strings.ToLower(s)
	var b strings.Builder
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' {
			b.WriteRune(r)
		} else {
			b.WriteRune('-')
		}
	}
	name := strings.Trim(b.String(), "-")
	if len(name) > 40 {
		name = name[:40]
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

func buildTestRun(name, namespace string, parallelism int, runnerImage, args string, envVarsMap map[string]string, defaultImage, defaultStarterImage string) *unstructured.Unstructured {
	if parallelism <= 0 {
		parallelism = 1
	}

	if runnerImage == "" {
		runnerImage = defaultImage
	}

	// Build env var list
	envVars := []any{}
	for k, v := range envVarsMap {
		envVars = append(envVars, map[string]any{
			"name":  k,
			"value": v,
		})
	}

	spec := map[string]any{
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
		spec["starter"] = map[string]any{
			"image": defaultStarterImage,
		}
	}

	if args != "" {
		spec["arguments"] = args
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
			"spec": spec,
		},
	}
}

func isTestFinished(phase string) bool {
	switch phase {
	case "finished", "errored", "error":
		return true
	}
	return false
}

func extractPhase(obj *unstructured.Unstructured) string {
	phase, _, _ := unstructured.NestedString(obj.Object, "status", "stage")
	if phase == "" {
		phase = "unknown"
	}
	return phase
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
