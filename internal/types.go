package internal

import (
	"fmt"
	"mime/multipart"

	validation "github.com/go-ozzo/ozzo-validation/v4"
	k8svalidation "k8s.io/apimachinery/pkg/util/validation"
)

// maxScriptBytes caps the size of an uploaded k6 script. Scripts are stored in
// a ConfigMap, which Kubernetes limits to 1MiB in total (data plus metadata),
// so leave some headroom for the object's metadata.
const maxScriptBytes = 1000 * 1000

type CreateTestRequest struct {
	Name          string                  `form:"name" json:"name"`
	Parallelism   int                     `form:"parallelism" json:"parallelism"`
	Script        *multipart.FileHeader   `form:"script" json:"script"`
	RunnerImage   string                  `form:"runnerImage" json:"runnerImage"`
	EnvVars       map[string]string       `form:"envVars" json:"envVars"`
	SecretEnvVars map[string]SecretKeyRef `form:"-" json:"secretEnvVars"`
	Args          string                  `form:"args" json:"args"`
}

// SecretKeyRef references a key in a Kubernetes Secret in the k6 namespace.
// The runner pods resolve the value themselves, so the secret's contents never
// pass through k6-manager or get stored on the TestRun.
type SecretKeyRef struct {
	Name string `json:"name"`
	Key  string `json:"key"`
}

func (r CreateTestRequest) Validate() error {
	return validation.ValidateStruct(&r,
		validation.Field(&r.Name, validation.Required),
		validation.Field(&r.Parallelism, validation.Required, validation.Min(1)),
		validation.Field(&r.Script, validation.Required, validation.By(validateScriptSize)),
		validation.Field(&r.SecretEnvVars, validation.By(r.validateSecretEnvVars)))
}

func validateScriptSize(value any) error {
	fh, ok := value.(*multipart.FileHeader)
	if !ok || fh == nil {
		return nil
	}
	if fh.Size > maxScriptBytes {
		return fmt.Errorf("must be no larger than %d bytes, got %d", maxScriptBytes, fh.Size)
	}
	return nil
}

func (r CreateTestRequest) validateSecretEnvVars(value any) error {
	refs, _ := value.(map[string]SecretKeyRef)
	for name, ref := range refs {
		if _, dup := r.EnvVars[name]; dup {
			return fmt.Errorf("%s is also defined as a plain environment variable", name)
		}
		if errs := k8svalidation.IsDNS1123Subdomain(ref.Name); len(errs) > 0 {
			return fmt.Errorf("%s: invalid secret name %q", name, ref.Name)
		}
		if errs := k8svalidation.IsConfigMapKey(ref.Key); len(errs) > 0 {
			return fmt.Errorf("%s: invalid secret key %q", name, ref.Key)
		}
	}
	return nil
}

type TestStatus struct {
	ID            string                  `json:"id"`
	Name          string                  `json:"name"`
	Namespace     string                  `json:"namespace"`
	Phase         string                  `json:"phase"`
	Finished      bool                    `json:"finished"`
	Result        string                  `json:"result,omitempty"`
	Parallelism   int                     `json:"parallelism"`
	StartedAt     string                  `json:"startedAt,omitempty"`
	FinishedAt    string                  `json:"finishedAt,omitempty"`
	StoppedAt     string                  `json:"stoppedAt,omitempty"`
	ConfigMap     string                  `json:"configMap"`
	Script        string                  `json:"scriptFile"`
	ScriptContent string                  `json:"script,omitempty"`
	RunnerImage   string                  `json:"runnerImage"`
	Args          string                  `json:"args"`
	EnvVars       map[string]string       `json:"envVars"`
	SecretEnvVars map[string]SecretKeyRef `json:"secretEnvVars"`
}

type ErrorResponse struct {
	Type     string              `json:"type,omitempty"`
	Title    string              `json:"title,omitempty"`
	Status   int                 `json:"status,omitempty"`
	Detail   string              `json:"detail,omitempty"`
	Instance string              `json:"instance,omitempty"`
	TraceID  string              `json:"traceId,omitempty"`
	Errors   map[string][]string `json:"errors,omitempty"`
}

func (e ErrorResponse) Error() string {
	return fmt.Sprintf("%s: %s", e.Title, e.Detail)
}
