package main

import (
	"context"
	"log/slog"
	"time"

	"github.com/labstack/echo/v5"

	"github.com/jkratz55/k6-manager/internal"
)

func main() {

	logger := internal.Logger()
	logger.Info("Initializing application")

	kubernetesClients, err := internal.InitKubernetesClient()
	if err != nil {
		logger.Error("Failed to initialize Kubernetes clients",
			slog.Any("error", err))
		panic(err)
	}
	logger.Info("Kubernetes clients initialized")

	config, err := internal.LoadConfig()
	if err != nil {
		logger.Error("Failed to load config",
			slog.Any("error", err))
		panic(err)
	}
	logger.Info("Config loaded")

	k6Service := internal.NewK6Service(kubernetesClients.Client, kubernetesClients.DynamicClient, config)
	logger.Info("K6 service initialized")

	e := echo.New()
	e.Logger = logger
	e.Validator = &internal.Validator{}
	e.HTTPErrorHandler = internal.ProblemDetailsErrorHandler

	handler := internal.NewHandler(k6Service)
	handler.RegisterRoutes(e)

	go func() {
		logger.Info("Starting background cleanup worker",
			slog.Duration("interval", config.CleanupInterval),
			slog.Duration("retention", config.TestRetention))
		ticker := time.NewTicker(config.CleanupInterval)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				logger.Info("Running background cleanup")
				if err := k6Service.CleanupTests(context.Background(), config.TestRetention); err != nil {
					logger.Error("Background cleanup failed", slog.Any("error", err))
				}
			}
		}
	}()

	logger.Info("Starting HTTP server on port 8080")
	if err := e.Start(":8080"); err != nil {
		logger.Error("Server failed to start or unexpectedly shutdown due to an error",
			slog.Any("error", err))
		panic(err)
	}

	logger.Info("Application exiting")
}
