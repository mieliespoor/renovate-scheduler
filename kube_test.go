package main

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func TestLoadEnvFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "task.env")
	content := "# comment\nFOO=bar\nEMPTY=\nSPACED = value with spaces\nNO_EQUALS\n\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write env file: %v", err)
	}

	vars, err := loadEnvFile(path)
	if err != nil {
		t.Fatalf("loadEnvFile: %v", err)
	}

	want := []corev1.EnvVar{
		{Name: "FOO", Value: "bar"},
		{Name: "EMPTY", Value: ""},
		{Name: "SPACED ", Value: " value with spaces"},
		{Name: "NO_EQUALS", Value: ""},
	}
	if !reflect.DeepEqual(vars, want) {
		t.Fatalf("loadEnvFile vars=%v, want %v", vars, want)
	}

	empty, err := loadEnvFile("")
	if err != nil {
		t.Fatalf("loadEnvFile empty path: %v", err)
	}
	if empty != nil {
		t.Fatalf("expected nil env vars for empty path")
	}
}

func TestBuildVolumeMountsAndVolumes(t *testing.T) {
	mounts, volumes, err := buildVolumeMountsAndVolumes([]RenovateVolumeMountConfig{
		{
			Name:      "host mount",
			Source:    "host_path",
			HostPath:  "/data",
			MountPath: "/mnt/host",
			ReadOnly:  true,
		},
		{
			Name:          "cm-mount",
			Source:        "config_map",
			ConfigMapName: "renovate-config",
			MountPath:     "/mnt/cm",
		},
		{
			Name:       "secret-mount",
			Source:     "secret",
			SecretName: "renovate-secret",
			MountPath:  "/mnt/secret",
		},
		{
			Name:                  "pvc mount",
			Source:                "persistent_volume_claim",
			PersistentVolumeClaim: "renovate-pvc",
			MountPath:             "/mnt/pvc",
			SubPath:               "subdir",
			ReadOnly:              true,
		},
	})
	if err != nil {
		t.Fatalf("buildVolumeMountsAndVolumes: %v", err)
	}
	if len(mounts) != 4 || len(volumes) != 4 {
		t.Fatalf("expected 4 mounts and volumes, got %d and %d", len(mounts), len(volumes))
	}

	if volumes[0].HostPath == nil || volumes[0].HostPath.Path != "/data" {
		t.Fatalf("expected hostPath volume")
	}
	if volumes[1].ConfigMap == nil || volumes[1].ConfigMap.Name != "renovate-config" {
		t.Fatalf("expected configMap volume")
	}
	if volumes[2].Secret == nil || volumes[2].Secret.SecretName != "renovate-secret" {
		t.Fatalf("expected secret volume")
	}
	if volumes[3].PersistentVolumeClaim == nil || volumes[3].PersistentVolumeClaim.ClaimName != "renovate-pvc" {
		t.Fatalf("expected pvc volume")
	}
	if mounts[3].SubPath != "subdir" {
		t.Fatalf("expected subPath to be set")
	}
}

func TestBuildVolumeMountsAndVolumesInvalidSource(t *testing.T) {
	_, _, err := buildVolumeMountsAndVolumes([]RenovateVolumeMountConfig{
		{
			Name:      "bad",
			Source:    "invalid",
			MountPath: "/mnt/bad",
		},
	})
	if err == nil {
		t.Fatalf("expected error for invalid mount source")
	}
}

func TestBuildJobIncludesTimeoutTTLAndEnv(t *testing.T) {
	cfg := &Config{
		Renovate: RenovateConfig{
			Image:         "renovate/renovate:latest",
			ContainerName: "renovate",
			VolumeMounts: []RenovateVolumeMountConfig{
				{
					Source:        "config_map",
					Name:          "renovate-config",
					MountPath:     "/tmp/renovate-config",
					ConfigMapName: "renovate-config",
				},
			},
		},
		Kubernetes: KubernetesConfig{
			Namespace:          "ns",
			ServiceAccountName: "svc",
			SecretName:         "renovate-secret",
		},
		Scheduler: SchedulerConfig{
			JobTimeoutSeconds: 120,
			JobTTLSeconds:     300,
		},
	}

	envVars := []corev1.EnvVar{{Name: "FOO", Value: "bar"}}
	job, err := buildJob(cfg, "org/repo", envVars)
	if err != nil {
		t.Fatalf("buildJob: %v", err)
	}

	if job.Namespace != "ns" {
		t.Fatalf("namespace=%q, want ns", job.Namespace)
	}
	if job.Spec.ActiveDeadlineSeconds == nil || *job.Spec.ActiveDeadlineSeconds != 120 {
		t.Fatalf("unexpected ActiveDeadlineSeconds: %v", job.Spec.ActiveDeadlineSeconds)
	}
	if job.Spec.TTLSecondsAfterFinished == nil || *job.Spec.TTLSecondsAfterFinished != 300 {
		t.Fatalf("unexpected TTLSecondsAfterFinished: %v", job.Spec.TTLSecondsAfterFinished)
	}

	container := job.Spec.Template.Spec.Containers[0]
	if container.Name == "renovate" {
		t.Fatalf("container name should be transformed to include repo and suffix")
	}
	if !strings.HasPrefix(container.Name, "renovate-") {
		t.Fatalf("container name %q should start with renovate-", container.Name)
	}
	if !reflect.DeepEqual(container.Env, envVars) {
		t.Fatalf("env vars=%v, want %v", container.Env, envVars)
	}
	if len(container.EnvFrom) == 0 || container.EnvFrom[0].SecretRef == nil || container.EnvFrom[0].SecretRef.Name != "renovate-secret" {
		t.Fatalf("expected EnvFrom secret reference")
	}
	if len(container.VolumeMounts) != 1 {
		t.Fatalf("expected one configured volume mount")
	}
	if len(job.Spec.Template.Spec.Volumes) != 1 {
		t.Fatalf("expected one configured volume")
	}
}

func TestPreflightRefs(t *testing.T) {
	cfg := &Config{
		Kubernetes: KubernetesConfig{Namespace: "renovate-scheduler", SecretName: "renovate-scheduler-secret"},
		Renovate: RenovateConfig{VolumeMounts: []RenovateVolumeMountConfig{
			{Source: "config_map", ConfigMapName: "renovate-config"},
			{Source: "host_path", HostPath: "/mnt/data"},
			{Source: "secret", SecretName: "extra-creds"},
			{Source: "pvc", PersistentVolumeClaim: "renovate-cache"},
		}},
	}

	want := []objectRef{
		{Kind: "secret", Name: "renovate-scheduler-secret", Field: "kubernetes.secret_name"},
		{Kind: "configmap", Name: "renovate-config", Field: "renovate.volume_mounts[0]"},
		{Kind: "secret", Name: "extra-creds", Field: "renovate.volume_mounts[2]"},
		{Kind: "persistentvolumeclaim", Name: "renovate-cache", Field: "renovate.volume_mounts[3]"},
	}
	if got := preflightRefs(cfg); !reflect.DeepEqual(got, want) {
		t.Fatalf("preflightRefs=%#v, want %#v", got, want)
	}
}

func TestPreflightRefsNoVolumeMounts(t *testing.T) {
	cfg := &Config{Kubernetes: KubernetesConfig{Namespace: "renovate-scheduler", SecretName: "renovate-scheduler-secret"}}
	want := []objectRef{{Kind: "secret", Name: "renovate-scheduler-secret", Field: "kubernetes.secret_name"}}
	if got := preflightRefs(cfg); !reflect.DeepEqual(got, want) {
		t.Fatalf("preflightRefs=%#v, want %#v", got, want)
	}
}

func TestPreflight(t *testing.T) {
	namespace := "renovate-scheduler"
	envPath := filepath.Join(t.TempDir(), "renovate-task.env")
	if err := os.WriteFile(envPath, []byte("RENOVATE_TOKEN=test\n"), 0o644); err != nil {
		t.Fatalf("write env file: %v", err)
	}

	cfg := &Config{
		Kubernetes: KubernetesConfig{Namespace: namespace, SecretName: "renovate-scheduler-secret"},
		Renovate: RenovateConfig{
			EnvFile: envPath,
			VolumeMounts: []RenovateVolumeMountConfig{
				{Source: "config_map", ConfigMapName: "renovate-config"},
				{Source: "secret", SecretName: "extra-credentials"},
				{Source: "persistent_volume_claim", PersistentVolumeClaim: "renovate-cache"},
			},
		},
	}
	client := fake.NewSimpleClientset(
		&corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "renovate-scheduler-secret", Namespace: namespace}},
		&corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "renovate-config", Namespace: namespace}},
		&corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "extra-credentials", Namespace: namespace}},
		&corev1.PersistentVolumeClaim{ObjectMeta: metav1.ObjectMeta{Name: "renovate-cache", Namespace: namespace}},
	)

	if err := preflight(t.Context(), client, cfg); err != nil {
		t.Fatalf("preflight() error = %v", err)
	}
}

func TestPreflightReportsAllMissingDependencies(t *testing.T) {
	cfg := &Config{
		Kubernetes: KubernetesConfig{Namespace: "renovate-scheduler", SecretName: "renovate-scheduler-secret"},
		Renovate: RenovateConfig{
			EnvFile: filepath.Join(t.TempDir(), "missing.env"),
			VolumeMounts: []RenovateVolumeMountConfig{
				{Source: "config_map", ConfigMapName: "renovate-config"},
				{Source: "persistent_volume_claim", PersistentVolumeClaim: "renovate-cache"},
			},
		},
	}

	err := preflight(t.Context(), fake.NewSimpleClientset(), cfg)
	if err == nil {
		t.Fatal("preflight() error = nil, want missing dependency error")
	}
	for _, field := range []string{
		"renovate.env_file:",
		"kubernetes.secret_name:",
		"renovate.volume_mounts[0]:",
		"renovate.volume_mounts[1]:",
	} {
		if !strings.Contains(err.Error(), field) {
			t.Errorf("preflight() error = %q, want field %q", err, field)
		}
	}
}
