package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"unicode"

	"github.com/BurntSushi/toml"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

const defaultMaxConcurrentTasks = 6
const defaultRunIntervalMinutes = 60
const defaultReposPollSeconds = 10
const defaultDispatchPollSeconds = 10
const defaultStateFilePath = "scheduler-state.json"
const defaultLogFormat = "text"

type Config struct {
	Renovate   RenovateConfig   `toml:"renovate"`
	Kubernetes KubernetesConfig `toml:"kubernetes"`
	Scheduler  SchedulerConfig  `toml:"scheduler"`
	Logging    LoggingConfig    `toml:"logging"`
}

type LoggingConfig struct {
	Format string `toml:"format"`
}

type RenovateConfig struct {
	Image         string                      `toml:"image"`
	ContainerName string                      `toml:"container_name"`
	EnvFile       string                      `toml:"env_file"`
	VolumeMounts  []RenovateVolumeMountConfig `toml:"volume_mounts"`
	Resources     RenovateResourcesConfig     `toml:"resources"`
	WorkloadJSON  string                      `toml:"workload_json"`
	Workload      RenovateWorkloadConfig      `toml:"-"`
}

// RenovateWorkloadConfig contains Kubernetes Job pod and container options.
// It is populated from workload_json so Helm can pass through native Kubernetes
// values without growing a parallel TOML schema for complex nested API types.
type RenovateWorkloadConfig struct {
	ImagePullPolicy    corev1.PullPolicy             `json:"imagePullPolicy"`
	ImagePullSecrets   []corev1.LocalObjectReference `json:"imagePullSecrets"`
	SecurityContext    *corev1.SecurityContext       `json:"securityContext"`
	PodSecurityContext *corev1.PodSecurityContext    `json:"podSecurityContext"`
	NodeSelector       map[string]string             `json:"nodeSelector"`
	Tolerations        []corev1.Toleration           `json:"tolerations"`
	Affinity           *corev1.Affinity              `json:"affinity"`
	PriorityClassName  string                        `json:"priorityClassName"`
}

// RenovateResourcesConfig maps resource names (cpu, memory, ephemeral-storage)
// to Kubernetes quantities for the Renovate Job container.
type RenovateResourcesConfig struct {
	Requests map[string]string `toml:"requests"`
	Limits   map[string]string `toml:"limits"`
}

type RenovateVolumeMountConfig struct {
	Name                  string `toml:"name"`
	Source                string `toml:"source"`
	HostPath              string `toml:"host_path"`
	ConfigMapName         string `toml:"config_map_name"`
	SecretName            string `toml:"secret_name"`
	PersistentVolumeClaim string `toml:"persistent_volume_claim"`
	MountPath             string `toml:"mount_path"`
	SubPath               string `toml:"sub_path"`
	ReadOnly              bool   `toml:"read_only"`
}

type KubernetesConfig struct {
	Namespace          string `toml:"namespace"`
	ServiceAccountName string `toml:"service_account_name"`
	SecretName         string `toml:"secret_name"`
	Kubeconfig         string `toml:"kubeconfig"`
}

type SchedulerConfig struct {
	MaxConcurrentTasks  int    `toml:"max_concurrent_tasks"`
	JobTimeoutSeconds   int    `toml:"job_timeout_seconds"`
	JobTTLSeconds       int    `toml:"job_ttl_seconds"`
	RunIntervalMinutes  int    `toml:"run_interval_minutes"`
	ReposPollSeconds    int    `toml:"repos_poll_seconds"`
	DispatchPollSeconds int    `toml:"dispatch_poll_seconds"`
	StateFilePath       string `toml:"state_file_path"`
}

func (m RenovateVolumeMountConfig) normalizedSource() (string, error) {
	source := strings.TrimSpace(strings.ToLower(m.Source))
	if source != "" {
		switch source {
		case "host_path", "config_map", "secret", "persistent_volume_claim", "pvc":
			return source, nil
		default:
			return "", fmt.Errorf("unsupported source %q", m.Source)
		}
	}

	if m.HostPath != "" {
		return "host_path", nil
	}
	if m.ConfigMapName != "" {
		return "config_map", nil
	}
	if m.SecretName != "" {
		return "secret", nil
	}
	if m.PersistentVolumeClaim != "" {
		return "persistent_volume_claim", nil
	}

	return "", nil
}

func (r RenovateResourcesConfig) build() (corev1.ResourceRequirements, error) {
	var out corev1.ResourceRequirements
	parse := func(kind string, in map[string]string) (corev1.ResourceList, error) {
		if len(in) == 0 {
			return nil, nil
		}
		list := corev1.ResourceList{}
		for name, value := range in {
			q, err := resource.ParseQuantity(value)
			if err != nil {
				return nil, fmt.Errorf("%s.%s %q: %w", kind, name, value, err)
			}
			list[corev1.ResourceName(name)] = q
		}
		return list, nil
	}
	var err error
	if out.Requests, err = parse("requests", r.Requests); err != nil {
		return out, err
	}
	if out.Limits, err = parse("limits", r.Limits); err != nil {
		return out, err
	}
	return out, nil
}

func (r *RenovateConfig) decodeWorkloadConfig() error {
	if strings.TrimSpace(r.WorkloadJSON) == "" {
		return nil
	}

	decoder := json.NewDecoder(strings.NewReader(r.WorkloadJSON))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&r.Workload); err != nil {
		return fmt.Errorf("renovate.workload_json: %w", err)
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		if err == nil {
			return fmt.Errorf("renovate.workload_json: contains multiple JSON values")
		}
		return fmt.Errorf("renovate.workload_json: %w", err)
	}

	switch r.Workload.ImagePullPolicy {
	case "", corev1.PullAlways, corev1.PullNever, corev1.PullIfNotPresent:
	default:
		return fmt.Errorf("renovate.workload_json: imagePullPolicy must be Always, Never, or IfNotPresent")
	}
	return nil
}

func isWindowsDrivePath(path string) bool {
	if len(path) < 3 {
		return false
	}
	return unicode.IsLetter(rune(path[0])) && path[1] == ':' && (path[2] == '\\' || path[2] == '/')
}

// LoadConfig reads and validates the TOML configuration at path, applying
// default for any unset option fields.
func loadConfig(path string) (*Config, error) {
	if _, err := os.Stat(path); err != nil {
		return nil, fmt.Errorf("config file %q: %w", path, err)
	}

	cfg := &Config{}
	meta, err := toml.DecodeFile(path, cfg)
	if err != nil {
		return nil, fmt.Errorf("parsing config %q: %w", path, err)
	}
	if undecoded := meta.Undecoded(); len(undecoded) > 0 {
		return nil, fmt.Errorf("config %q contains unknown keys: %v", path, undecoded)
	}

	if cfg.Scheduler.MaxConcurrentTasks <= 0 {
		cfg.Scheduler.MaxConcurrentTasks = defaultMaxConcurrentTasks
	}
	if cfg.Scheduler.RunIntervalMinutes <= 0 {
		cfg.Scheduler.RunIntervalMinutes = defaultRunIntervalMinutes
	}
	if cfg.Scheduler.ReposPollSeconds <= 0 {
		cfg.Scheduler.ReposPollSeconds = defaultReposPollSeconds
	}
	if cfg.Scheduler.DispatchPollSeconds <= 0 {
		cfg.Scheduler.DispatchPollSeconds = defaultDispatchPollSeconds
	}
	if cfg.Scheduler.StateFilePath == "" {
		cfg.Scheduler.StateFilePath = defaultStateFilePath
	}
	if cfg.Logging.Format == "" {
		cfg.Logging.Format = defaultLogFormat
	}
	if cfg.Kubernetes.Namespace == "" {
		cfg.Kubernetes.Namespace = "default"
	}
	if cfg.Renovate.ContainerName == "" {
		cfg.Renovate.ContainerName = "renovate"
	}

	if cfg.Renovate.Image == "" {
		return nil, fmt.Errorf("config %q: renovate.image must be set", path)
	}
	if err := cfg.Renovate.decodeWorkloadConfig(); err != nil {
		return nil, fmt.Errorf("config %q: %w", path, err)
	}

	if cfg.Kubernetes.SecretName == "" {
		return nil, fmt.Errorf("config %q: kubernetes.secret_name must be set", path)
	}

	cfg.Logging.Format = strings.TrimSpace(strings.ToLower(cfg.Logging.Format))
	if cfg.Logging.Format != "text" && cfg.Logging.Format != "json" {
		return nil, fmt.Errorf("config %q: logging.format must be either text or json", path)
	}

	if _, err := cfg.Renovate.Resources.build(); err != nil {
		return nil, fmt.Errorf("config %q: renovate.resources: %w", path, err)
	}

	for i, mount := range cfg.Renovate.VolumeMounts {
		source, err := mount.normalizedSource()
		if err != nil {
			return nil, fmt.Errorf("config %q: renovate.volume_mounts[%d]: %w", path, i, err)
		}
		if mount.MountPath == "" {
			return nil, fmt.Errorf("config %q: renovate.volume_mounts[%d].mount_path must be set", path, i)
		}

		switch source {
		case "host_path":
			if mount.HostPath == "" {
				return nil, fmt.Errorf("config %q: renovate.volume_mounts[%d].host_path must be set for source=host_path", path, i)
			}
			if isWindowsDrivePath(mount.HostPath) {
				return nil, fmt.Errorf("config %q: renovate.volume_mounts[%d].host_path %q is a Windows path; hostPath must be a Linux absolute path visible to the Kubernetes node (for Docker Desktop + Minikube, try /run/desktop/mnt/host/c/...); or use source=config_map/secret", path, i, mount.HostPath)
			}
		case "config_map":
			if mount.ConfigMapName == "" {
				return nil, fmt.Errorf("config %q: renovate.volume_mounts[%d].config_map_name must be set for source=config_map", path, i)
			}
		case "secret":
			if mount.SecretName == "" {
				return nil, fmt.Errorf("config %q: renovate.volume_mounts[%d].secret_name must be set for source=secret", path, i)
			}
		case "persistent_volume_claim", "pvc":
			if mount.PersistentVolumeClaim == "" {
				return nil, fmt.Errorf("config %q: renovate.volume_mounts[%d].persistent_volume_claim must be set for source=persistent_volume_claim", path, i)
			}
		default:
			return nil, fmt.Errorf("config %q: renovate.volume_mounts[%d] must define a source (host_path, config_map, secret, persistent_volume_claim)", path, i)
		}
	}

	return cfg, nil
}
