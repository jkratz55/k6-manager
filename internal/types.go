package internal

import (
	"fmt"
	"mime/multipart"

	validation "github.com/go-ozzo/ozzo-validation/v4"
)

type CreateTestRequest struct {
	Name        string                `form:"name" json:"name"`
	Parallelism int                   `form:"parallelism" json:"parallelism"`
	Script      *multipart.FileHeader `form:"script" json:"script"`
	RunnerImage string                `form:"runnerImage" json:"runnerImage"`
	EnvVars     map[string]string     `form:"envVars" json:"envVars"`
	Args        string                `form:"args" json:"args"`
}

func (r CreateTestRequest) Validate() error {
	return validation.ValidateStruct(&r,
		validation.Field(&r.Name, validation.Required),
		validation.Field(&r.Parallelism, validation.Required, validation.Min(1)),
		validation.Field(&r.Script, validation.Required))
}

type TestStatus struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Namespace   string `json:"namespace"`
	Phase       string `json:"phase"`
	Parallelism int    `json:"parallelism"`
	StartedAt   string `json:"startedAt,omitempty"`
	FinishedAt  string `json:"finishedAt,omitempty"`
	ConfigMap   string `json:"configMap"`
	Script      string `json:"scriptFile"`
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
