package main

import (
	"fmt"
	"os"
	"strings"
	"unicode"

	"github.com/BurntSushi/toml"
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

	if cfg.Kubernetes.SecretName == "" {
		return nil, fmt.Errorf("config %q: kubernetes.secret_name must be set", path)
	}

	cfg.Logging.Format = strings.TrimSpace(strings.ToLower(cfg.Logging.Format))
	if cfg.Logging.Format != "text" && cfg.Logging.Format != "json" {
		return nil, fmt.Errorf("config %q: logging.format must be either text or json", path)
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
