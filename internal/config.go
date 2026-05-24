package internal

import (
	"context"
	"fmt"

	"github.com/sethvargo/go-envconfig"
)

type AppConfig struct {
	Namespace          string `env:"K6_NAMESPACE, default=k6"`
	DefaultRunnerImage string `env:"K6_DEFAULT_RUNNER_IMAGE, default=docker.io/grafana/k6:2.0.0"`
}

func LoadConfig() (AppConfig, error) {
	var config AppConfig
	if err := envconfig.Process(context.Background(), &config); err != nil {
		return AppConfig{}, fmt.Errorf("load config: %w", err)
	}
	return config, nil
}
