package internal

import (
	"mime/multipart"

	validation "github.com/go-ozzo/ozzo-validation/v4"
)

type CreateTestRequest struct {
	Name        string                `form:"name"`
	Parallelism int                   `form:"parallelism"`
	Script      *multipart.FileHeader `form:"script"`
	RunnerImage string                `form:"runnerImage"`
	EnvVars     map[string]string     `form:"envVars"`
	Args        string                `form:"args"`
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
	Code    int    `json:"code"`
	Type    string `json:"type"`
	Message string `json:"message"`
	Detail  string `json:"detail"`
	Errors  map[string][]string
}
