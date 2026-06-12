package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNormalizedSource(t *testing.T) {
	tests := []struct {
		name    string
		mount   RenovateVolumeMountConfig
		want    string
		wantErr bool
	}{
		{name: "explicit host_path", mount: RenovateVolumeMountConfig{Source: "host_path"}, want: "host_path"},
		{name: "explicit pvc alias", mount: RenovateVolumeMountConfig{Source: "pvc"}, want: "pvc"},
		{name: "infer host_path", mount: RenovateVolumeMountConfig{HostPath: "/data"}, want: "host_path"},
		{name: "infer config_map", mount: RenovateVolumeMountConfig{ConfigMapName: "cm"}, want: "config_map"},
		{name: "infer secret", mount: RenovateVolumeMountConfig{SecretName: "s"}, want: "secret"},
		{name: "infer pvc", mount: RenovateVolumeMountConfig{PersistentVolumeClaim: "claim"}, want: "persistent_volume_claim"},
		{name: "unknown explicit", mount: RenovateVolumeMountConfig{Source: "unknown"}, wantErr: true},
		{name: "no source", mount: RenovateVolumeMountConfig{}, want: ""},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := tc.mount.normalizedSource()
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tc.want {
				t.Fatalf("got %q, want %q", got, tc.want)
			}
		})
	}
}

func TestIsWindowsDrivePath(t *testing.T) {
	tests := []struct {
		path string
		want bool
	}{
		{path: `C:\Users\me`, want: true},
		{path: `D:/work`, want: true},
		{path: `/mnt/c/Users/me`, want: false},
		{path: `relative/path`, want: false},
		{path: ``, want: false},
	}

	for _, tc := range tests {
		if got := isWindowsDrivePath(tc.path); got != tc.want {
			t.Fatalf("isWindowsDrivePath(%q)=%v, want %v", tc.path, got, tc.want)
		}
	}
}

func TestLoadConfigAppliesDefaults(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.toml")
	content := `
[renovate]
image = "renovate/renovate:latest"

[kubernetes]
secret_name = "renovate-secret"
`
	if err := os.WriteFile(cfgPath, []byte(content), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cfg, err := loadConfig(cfgPath)
	if err != nil {
		t.Fatalf("loadConfig: %v", err)
	}

	if cfg.Scheduler.MaxConcurrentTasks != defaultMaxConcurrentTasks {
		t.Fatalf("MaxConcurrentTasks=%d, want %d", cfg.Scheduler.MaxConcurrentTasks, defaultMaxConcurrentTasks)
	}
	if cfg.Scheduler.RunIntervalMinutes != defaultRunIntervalMinutes {
		t.Fatalf("RunIntervalMinutes=%d, want %d", cfg.Scheduler.RunIntervalMinutes, defaultRunIntervalMinutes)
	}
	if cfg.Scheduler.ReposPollSeconds != defaultReposPollSeconds {
		t.Fatalf("ReposPollSeconds=%d, want %d", cfg.Scheduler.ReposPollSeconds, defaultReposPollSeconds)
	}
	if cfg.Scheduler.DispatchPollSeconds != defaultDispatchPollSeconds {
		t.Fatalf("DispatchPollSeconds=%d, want %d", cfg.Scheduler.DispatchPollSeconds, defaultDispatchPollSeconds)
	}
	if cfg.Scheduler.StateFilePath != defaultStateFilePath {
		t.Fatalf("StateFilePath=%q, want %q", cfg.Scheduler.StateFilePath, defaultStateFilePath)
	}
	if cfg.Kubernetes.Namespace != "default" {
		t.Fatalf("Namespace=%q, want default", cfg.Kubernetes.Namespace)
	}
	if cfg.Renovate.ContainerName != "renovate" {
		t.Fatalf("ContainerName=%q, want renovate", cfg.Renovate.ContainerName)
	}
	if cfg.Logging.Format != defaultLogFormat {
		t.Fatalf("Logging.Format=%q, want %q", cfg.Logging.Format, defaultLogFormat)
	}
}

func TestLoadConfigValidatesRequiredFields(t *testing.T) {
	dir := t.TempDir()

	missingImage := filepath.Join(dir, "missing-image.toml")
	if err := os.WriteFile(missingImage, []byte("[kubernetes]\nsecret_name=\"s\"\n"), 0o644); err != nil {
		t.Fatalf("write missing-image config: %v", err)
	}
	if _, err := loadConfig(missingImage); err == nil || !strings.Contains(err.Error(), "renovate.image") {
		t.Fatalf("expected renovate.image error, got %v", err)
	}

	missingSecret := filepath.Join(dir, "missing-secret.toml")
	if err := os.WriteFile(missingSecret, []byte("[renovate]\nimage=\"renovate/renovate:latest\"\n"), 0o644); err != nil {
		t.Fatalf("write missing-secret config: %v", err)
	}
	if _, err := loadConfig(missingSecret); err == nil || !strings.Contains(err.Error(), "kubernetes.secret_name") {
		t.Fatalf("expected kubernetes.secret_name error, got %v", err)
	}
}

func TestLoadConfigVolumeMountValidation(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.toml")
	content := `
[renovate]
image = "renovate/renovate:latest"

[[renovate.volume_mounts]]
source = "host_path"
host_path = "C:/Users/me/.renovate"
mount_path = "/tmp/renovate-config"

[kubernetes]
secret_name = "renovate-secret"
`
	if err := os.WriteFile(cfgPath, []byte(content), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	_, err := loadConfig(cfgPath)
	if err == nil {
		t.Fatalf("expected error for windows host_path")
	}
	if !strings.Contains(err.Error(), "Windows path") {
		t.Fatalf("expected windows path validation error, got %v", err)
	}
}

func TestLoadConfigLoggingFormatValidation(t *testing.T) {
	dir := t.TempDir()

	validPath := filepath.Join(dir, "valid-logging.toml")
	validContent := `
[renovate]
image = "renovate/renovate:latest"

[kubernetes]
secret_name = "renovate-secret"

[logging]
format = "JSON"
`
	if err := os.WriteFile(validPath, []byte(validContent), 0o644); err != nil {
		t.Fatalf("write valid logging config: %v", err)
	}

	cfg, err := loadConfig(validPath)
	if err != nil {
		t.Fatalf("expected valid logging format, got error: %v", err)
	}
	if cfg.Logging.Format != "json" {
		t.Fatalf("Logging.Format=%q, want json", cfg.Logging.Format)
	}

	invalidPath := filepath.Join(dir, "invalid-logging.toml")
	invalidContent := `
[renovate]
image = "renovate/renovate:latest"

[kubernetes]
secret_name = "renovate-secret"

[logging]
format = "yaml"
`
	if err := os.WriteFile(invalidPath, []byte(invalidContent), 0o644); err != nil {
		t.Fatalf("write invalid logging config: %v", err)
	}

	if _, err := loadConfig(invalidPath); err == nil || !strings.Contains(err.Error(), "logging.format") {
		t.Fatalf("expected logging.format validation error, got %v", err)
	}
}
