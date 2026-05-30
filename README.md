# K6 Manager

K6 Manager is a web application designed to simplify the management and execution of k6 load tests within a Kubernetes environment. It provides a user-friendly interface for creating, monitoring, and managing distributed k6 tests. By leveraging the [k6-operator](https://github.com/grafana/k6-operator), K6 Manager automates the process of deploying k6 runners and collecting results.

## Overview

K6 Manager serves as a management layer on top of the k6-operator. It provides:

- **Web UI**: A modern React-based interface to create and monitor tests.
- **Automated Lifecycle**: Handles the creation of Kubernetes ConfigMaps for scripts and the corresponding `TestRun` resources.
- **Cleanup Worker**: A background process that automatically removes old tests based on a configurable retention policy.
- **Extensible Configuration**: Easily configure namespaces, default runner images, and retention periods.

## Installation

### 2A. Running the application in Docker

You can build and run K6 Manager as a Docker container.

**Prerequisites:**
- Docker installed.
- A running Kubernetes cluster.
- Valid Kubernetes credentials (the container uses the standard kubeconfig or in-cluster config).

**Building the Image:**
```bash
make docker-build
```
Or directly via Docker:
```bash
docker build -t k6-manager:latest .
```

**Running the Container:**
To run the container locally and connect to your cluster, you can mount your kubeconfig:
```bash
docker run -p 8080:8080 -v ~/.kube/config:/home/nonroot/.kube/config k6-manager:latest
```

### 2B. Deploying with the Helm Chart

For production environments, it is recommended to deploy K6 Manager using the provided Helm chart.

**Prerequisites:**
- Helm 3+ installed.
- [k6-operator](https://github.com/grafana/k6-operator) installed in your cluster.

**Installation:**
```bash
helm install k6-manager ./chart -n k6 --create-namespace
```

**Customizing the Installation:**
You can override default values using a custom `values.yaml` or via `--set` flags. See the [Configuration](#configuration) section for available options.

## Configuration

The application can be configured using environment variables. When deploying via Helm, these are mapped to the `k6` section in `values.yaml`.

| Variable | Description | Default |
|----------|-------------|---------|
| `K6_NAMESPACE` | The Kubernetes namespace where k6 tests will be executed. | `k6` |
| `K6_DEFAULT_RUNNER_IMAGE` | The default Docker image used for k6 runners. | `docker.io/grafana/k6:latest` |
| `CLEANUP_INTERVAL` | How often the background cleanup worker runs. | `1h` |
| `TEST_RETENTION` | How long tests are kept before being automatically deleted. | `168h` |

## Application Structure

- `main.go`: The entry point of the Go application.
- `internal/`: Contains the backend logic, including Kubernetes interaction and API handlers.
    - `config.go`: Configuration loading and structure.
    - `k6.go`: Core logic for managing k6 `TestRun` resources.
    - `kubernetes.go`: Kubernetes client initialization.
    - `http.go`: REST API route definitions and handlers.
- `frontend/`: A React-based web interface built with Vite and Tailwind CSS.
    - `src/pages/`: Contains the main UI views (Create Test, Test List, Test Detail).
    - `src/api/`: API client for interacting with the backend.
- `chart/`: The Helm chart for deploying K6 Manager to Kubernetes.
- `Dockerfile`: Multi-stage Dockerfile for building both the frontend and backend into a single distroless image.
- `Makefile`: Contains common development and build commands (build, test, lint, docker-build).