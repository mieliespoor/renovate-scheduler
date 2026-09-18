# Renovate Scheduler

A Kubernetes-based scheduler that orchestrates Renovate jobs across multiple repositories. This application continuously monitors a repositories JSON file, persists repository run state, and creates Kubernetes Jobs with configurable concurrency and run intervals.

## Features

- **Kubernetes-native**: Runs Renovate as Kubernetes Jobs
- **Configurable concurrency**: Control the number of parallel Renovate jobs
- **Repository management**: Define repositories in a JSON file
- **Continuous ingestion**: Watches and ingests repositories from JSON on an interval
- **Persistent scheduling state**: Tracks last execution times in a JSON state store
- **TOML configuration**: Simple and flexible configuration format
- **Graceful shutdown**: Handles SIGTERM and SIGINT signals
- **Job tracking**: Monitors job execution and reports results

## Prerequisites

- **Go** 1.26+ (for development)
- **Docker** (for building the container image)
- **Minikube** (for local Kubernetes testing)
- **kubectl** (for interacting with Kubernetes)

## Install with Helm

The chart is published as an OCI artifact to GitHub Container Registry. Replace
`<version>` with a published chart version (without the `v` prefix).

```bash
helm install renovate-scheduler \
  oci://ghcr.io/mieliespoor/charts/renovate-scheduler \
  --version <version> \
  --namespace renovate-scheduler \
  --create-namespace
```

## Local Setup with Minikube

### 1. Install Minikube

If you haven't already installed Minikube, follow the [official installation guide](https://minikube.sigs.k8s.io/docs/start/).

### 2. Start Minikube

```bash
minikube start --cpus=4 --memory=8192
```

This allocates 4 CPUs and 8GB of memory to the cluster. Adjust based on your system resources.

### 3. Enable Docker Registry Addon (Optional)

To use a local Docker registry with Minikube:

```bash
minikube addons enable registry
```

### 4. Configure kubectl

```bash
kubectl config use-context minikube
```

Verify the connection:

```bash
kubectl cluster-info
kubectl get nodes
```

### 5. Use Minikube kubeconfig in debug runs

If you run the app from VS Code debug, set `kubernetes.kubeconfig` in `config.toml` to your Minikube kubeconfig path, or set `KUBECONFIG` in the debug environment.

On Windows, the default path is usually `C:\Users\<you>\.kube\config` after `minikube start` and `kubectl config use-context minikube`.

## Building and Testing Locally

### 1. Build the Docker Image

#### Option A: Build locally and push to Minikube

```bash
# Build the image
docker build -t renovatescheduler:dev .

# Tag it for Minikube's registry
docker tag renovatescheduler:dev $(minikube ip):5000/renovatescheduler:dev

# Push to Minikube registry (if enabled)
docker push $(minikube ip):5000/renovatescheduler:dev
```

#### Option B: Build directly in Minikube's Docker daemon

```bash
# Point Docker to Minikube's daemon
eval $(minikube docker-env)

# Build the image
docker build -t renovatescheduler:dev .

# Reset Docker context
eval $(minikube docker-env --unset)
```

### 2. Create a Kubernetes Namespace

```bash
kubectl create namespace renovate-scheduler
```

### 3. Create Required Secrets

Create a secret with your Renovate configuration (GitHub token, GitLab token, etc.):

```bash
kubectl create secret generic renovate-scheduler-secret \
  -n renovate-scheduler \
  --from-literal=RENOVATE_TOKEN=your_github_token \
  --from-literal=RENOVATE_LOG_LEVEL=debug
```

For local testing, you can use a dummy token:

```bash
kubectl create secret generic renovate-scheduler-secret \
  -n renovate-scheduler \
  --from-literal=RENOVATE_TOKEN=test_token \
  --from-literal=RENOVATE_LOG_LEVEL=debug
```

### 4. Create a Service Account

```bash
kubectl create serviceaccount renovate-scheduler -n renovate-scheduler
```

### 5. Create RBAC Roles

```bash
# Create ClusterRole for Job management
kubectl create clusterrole renovate-scheduler-role \
  --verb=create,get,list,watch,delete \
  --resource=jobs,jobs/status \
  -n renovate-scheduler

# Bind the role to the service account
kubectl create clusterrolebinding renovate-scheduler-binding \
  --clusterrole=renovate-scheduler-role \
  --serviceaccount=renovate-scheduler:renovate-scheduler
```

### 6. Prepare Configuration Files

Update `config.toml` for local testing:

```toml
[renovate]
  image = "renovate/renovate:latest"
  container_name = "renovate"

[kubernetes]
  namespace = "renovate-scheduler"
  service_account_name = "renovate-scheduler"
  secret_name = "renovate-scheduler-secret"
  kubeconfig = ""  # Empty to use in-cluster config or default kubeconfig

[scheduler]
  max_concurrent_tasks = 2
  job_timeout_seconds = 300
  job_ttl_seconds = 600
  run_interval_minutes = 60
  repos_poll_seconds = 10
  dispatch_poll_seconds = 10
  state_file_path = "scheduler-state.json"

[logging]
  format = "text"  # supported: text, json
```

Create or update `renovate-repos.json` with test repositories:

```json
[
  "owner/repo1",
  "owner/repo2"
]
```

### 7. Deploy the Application

You can deploy the scheduler as a Kubernetes Job or Pod for testing:

```yaml
apiVersion: batch/v1
kind: Job
metadata:
  name: renovate-scheduler-test
  namespace: renovate-scheduler
spec:
  template:
    metadata:
      name: renovate-scheduler-test
    spec:
      serviceAccountName: renovate-scheduler
      containers:
      - name: renovate-scheduler
        image: renovatescheduler:dev
        imagePullPolicy: IfNotPresent
        args:
          - -config=/app/config.toml
          - -repos=/app/renovate-repos.json
        volumeMounts:
        - name: config
          mountPath: /app/config.toml
          subPath: config.toml
        - name: repos
          mountPath: /app/renovate-repos.json
          subPath: renovate-repos.json
        env:
        - name: RENOVATE_TOKEN
          valueFrom:
            secretKeyRef:
              name: renovate-scheduler-secret
              key: RENOVATE_TOKEN
        - name: RENOVATE_LOG_LEVEL
          valueFrom:
            secretKeyRef:
              name: renovate-scheduler-secret
              key: RENOVATE_LOG_LEVEL
      volumes:
      - name: config
        configMap:
          name: renovate-scheduler-config
      - name: repos
        configMap:
          name: renovate-scheduler-repos
      restartPolicy: Never
  backoffLimit: 1
```

Save this as `test-job.yaml`, then create a ConfigMap for your files:

```bash
kubectl create configmap renovate-scheduler-config \
  -n renovate-scheduler \
  --from-file=config.toml=./config.toml

kubectl create configmap renovate-scheduler-repos \
  -n renovate-scheduler \
  --from-file=renovate-repos.json=./renovate-repos.json
```

Deploy the test job:

```bash
kubectl apply -f test-job.yaml
```

### 8. Monitor the Job

```bash
# Watch the job status
kubectl get jobs -n renovate-scheduler -w

# View logs
kubectl logs -n renovate-scheduler -l job-name=renovate-scheduler-test

# Get detailed job info
kubectl describe job renovate-scheduler-test -n renovate-scheduler
```

## Testing Locally

### Run Tests

```bash
go test ./...
```

### Run the Application Directly (Outside Kubernetes)

For quick testing outside Kubernetes:

```bash
# Install dependencies
go mod download

# Run the scheduler
go run . -config=config.toml -repos=renovate-repos.json
```

**Note**: Running outside Kubernetes requires a valid kubeconfig. The application will automatically detect and use your current kubectl context.

### Debug Mode

To see verbose logging, set the log level in your secret:

```bash
kubectl create secret generic renovate-scheduler-secret \
  -n renovate-scheduler \
  --from-literal=RENOVATE_TOKEN=test_token \
  --from-literal=RENOVATE_LOG_LEVEL=trace \
  --dry-run=client -o yaml | kubectl apply -f -
```

## Configuration

### config.toml

| Section | Key | Description | Required |
|---------|-----|-------------|----------|
| `renovate.image` | Docker image for Renovate | Yes |
| `renovate.container_name` | Container name in the pod | No (default: "renovate") |
| `renovate.env_file` | Path to a `.env` file injected into container `Env` | No |
| `renovate.volume_mounts` | List of mounts for Renovate container (`host_path`, `config_map`, `secret`, `persistent_volume_claim`) | No |
| `kubernetes.namespace` | Kubernetes namespace | No (default: "default") |
| `kubernetes.service_account_name` | Service account for jobs | No |
| `kubernetes.secret_name` | Secret containing credentials | Yes |
| `kubernetes.kubeconfig` | Path to kubeconfig file | No (auto-detected) |
| `scheduler.max_concurrent_tasks` | Max parallel jobs | No (default: 6) |
| `scheduler.run_interval_minutes` | Minimum time between runs per repository | No (default: 60) |
| `scheduler.repos_poll_seconds` | Poll interval for reading `renovate-repos.json` | No (default: 10) |
| `scheduler.dispatch_poll_seconds` | Poll interval for dispatching due jobs | No (default: 10) |
| `scheduler.state_file_path` | Path to persistent repository run state JSON file | No (default: `scheduler-state.json`) |
| `scheduler.job_timeout_seconds` | Job timeout in seconds | No |
| `scheduler.job_ttl_seconds` | Job TTL in seconds | No |
| `logging.format` | Application log output format (`text` or `json`) | No (default: `text`) |

### Structured Logging

The scheduler emits structured logs and supports two output formats:

- `text`: human-readable key/value format for local debugging
- `json`: machine-readable JSON for log aggregation platforms

Set the format in `config.toml`:

```toml
[logging]
  format = "json"
```

Example `text` log line:

```text
time=2026-06-12T21:10:11.123Z level=INFO msg="dispatch repository run completed" repository=owner/repo duration=14s
```

Example `json` log line:

```json
{"time":"2026-06-12T21:10:11.123Z","level":"INFO","msg":"dispatch repository run completed","repository":"owner/repo","duration":"14s"}
```

Example mount config for Renovate `config.js`:

```toml
[[renovate.volume_mounts]]
  name = "renovate-config"
  source = "host_path"
  host_path = "C:/Users/<you>/.renovate"
  mount_path = "/usr/src/app/config"
  read_only = true
```

ConfigMap mount:

```toml
[[renovate.volume_mounts]]
  name = "renovate-configmap"
  source = "config_map"
  config_map_name = "renovate-config"
  mount_path = "/usr/src/app/config"
  read_only = true
```

Secret mount:

```toml
[[renovate.volume_mounts]]
  name = "renovate-secret"
  source = "secret"
  secret_name = "renovate-secret-files"
  mount_path = "/usr/src/app/secret"
  read_only = true
```

PersistentVolumeClaim mount:

```toml
[[renovate.volume_mounts]]
  name = "renovate-pvc"
  source = "persistent_volume_claim"
  persistent_volume_claim = "renovate-config-pvc"
  mount_path = "/usr/src/app/config"
  sub_path = "renovate"
  read_only = false
```

### renovate-repos.json

Simple JSON array of repository references:

```json
[
  "github.com/owner/repo1",
  "github.com/owner/repo2",
  "gitlab.com/owner/repo3"
]
```

## Troubleshooting

### Issue: Image pull errors in Minikube

**Solution**: Build the image directly in Minikube's Docker daemon:

```bash
eval $(minikube docker-env)
docker build -t renovatescheduler:dev .
eval $(minikube docker-env --unset)
```

Or use `imagePullPolicy: IfNotPresent` in your deployment.

### Issue: Permission denied errors

**Solution**: Ensure the service account has proper RBAC bindings:

```bash
kubectl get rolebindings -n renovate-scheduler
kubectl get clusterrolebindings | grep renovate
```

### Issue: Jobs not starting

**Solution**: Check logs for configuration issues:

```bash
# Check the scheduler pod logs
kubectl logs -n renovate-scheduler <pod-name>

# Check created jobs
kubectl get jobs -n renovate-scheduler -o wide
```

### Issue: Minikube cluster not responding

**Solution**: Restart Minikube:

```bash
minikube stop
minikube start
```

## Development

### Project Structure

- `main.go` - Application entry point and main logic
- `config.go` - Configuration loading and validation
- `kube.go` - Kubernetes client and job management
- `config.toml` - Configuration template
- `renovate-repos.json` - Repository list
- `Dockerfile` - Multi-stage build configuration

### Building

```bash
# Build for Linux
CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o renovate-scheduler .

# Build for local machine
go build -o renovate-scheduler .
```

### Dependencies

- `github.com/BurntSushi/toml` - TOML configuration parsing
- `k8s.io/api` - Kubernetes API definitions
- `k8s.io/client-go` - Kubernetes Go client

## License

[Add your license here]
