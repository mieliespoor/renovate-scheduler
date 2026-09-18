package main

import (
	"bufio"
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"regexp"
	"strings"
	"time"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

type kubeClient = kubernetes.Interface

const jobPollInterval = 5 * time.Second

const maxJobNameLen = 63

func newClient(cfg *Config) (*kubernetes.Clientset, error) {
	restConfig, err := buildRestConfig(cfg.Kubernetes.Kubeconfig)
	if err != nil {
		return nil, err
	}
	return kubernetes.NewForConfig(restConfig)
}

func buildRestConfig(kubeconfigPath string) (*rest.Config, error) {
	if kubeconfigPath != "" {
		return clientcmd.BuildConfigFromFlags("", kubeconfigPath)
	}

	// Fall back to the standard kubeconfig loading rules, which honor KUBECONFIG
	// and the default kubeconfig location.
	rules := clientcmd.NewDefaultClientConfigLoadingRules()
	overrides := &clientcmd.ConfigOverrides{}
	config, err := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(rules, overrides).ClientConfig()
	if err == nil {
		return config, nil
	}

	return rest.InClusterConfig()
}

var runRenovateJob = func(ctx context.Context, client interface{}, cfg *Config, repo string) error {
	if client == nil {
		return fmt.Errorf("kubernetes client is nil")
	}
	k8sClient := client.(kubernetes.Interface)
	return defaultRunRenovateJob(ctx, k8sClient, cfg, repo)
}

func defaultRunRenovateJob(ctx context.Context, client kubernetes.Interface, cfg *Config, repo string) error {
	envVars, err := loadEnvFile(cfg.Renovate.EnvFile)
	if err != nil {
		return fmt.Errorf("loading env file: %w", err)
	}
	job, err := buildJob(cfg, repo, envVars)
	if err != nil {
		return err
	}

	created, err := client.BatchV1().Jobs(cfg.Kubernetes.Namespace).Create(ctx, job, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("failed to create job: %w", err)
	}
	name := created.Name

	if err := waitForJob(ctx, client, cfg.Kubernetes.Namespace, name); err != nil {
		return err
	}
	return nil
}

func waitForJob(ctx context.Context, client kubernetes.Interface, namespace, name string) error {
	return wait.PollUntilContextCancel(ctx, jobPollInterval, true, func(ctx context.Context) (bool, error) {
		job, err := client.BatchV1().Jobs(namespace).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			// Transient API errors should not abort the whole run; keep polling.
			return false, nil
		}
		for _, c := range job.Status.Conditions {
			if c.Status != corev1.ConditionTrue {
				continue
			}
			switch c.Type {
			case batchv1.JobComplete:
				return true, nil
			case batchv1.JobFailed:
				reason := c.Reason
				if c.Message != "" {
					reason = fmt.Sprintf("%s: %s", reason, c.Message)
				}
				return true, fmt.Errorf("job %q failed: (%s)", name, reason)
			}
		}
		return false, nil
	})
}

// objectRef is a Kubernetes object the scheduler will name when it builds a
// Renovate Job. Field records which config option asked for it, so a failure
// tells the user what to fix rather than just the missing object.
type objectRef struct {
	Kind  string
	Name  string
	Field string
}

// preflightRefs lists every Kubernetes object that buildJob will reference.
// Keeping it separate from the API calls makes the list easy to test without a cluster.
func preflightRefs(cfg *Config) []objectRef {
	refs := []objectRef{
		{Kind: "secret", Name: cfg.Kubernetes.SecretName, Field: "kubernetes.secret_name"},
	}

	for index, mount := range cfg.Renovate.VolumeMounts {
		field := fmt.Sprintf("renovate.volume_mounts[%d]", index)
		source, err := mount.normalizedSource()
		if err != nil {
			continue
		}

		switch source {
		case "config_map":
			refs = append(refs, objectRef{Kind: "configmap", Name: mount.ConfigMapName, Field: field})
		case "secret":
			refs = append(refs, objectRef{Kind: "secret", Name: mount.SecretName, Field: field})
		case "persistent_volume_claim", "pvc":
			refs = append(refs, objectRef{Kind: "persistentvolumeclaim", Name: mount.PersistentVolumeClaim, Field: field})
		}
	}

	return refs
}

// preflight checks that all files and Kubernetes objects required by a Renovate
// Job exist before the first repository is dispatched.
func preflight(ctx context.Context, client kubernetes.Interface, cfg *Config) error {
	var problems []string

	if cfg.Renovate.EnvFile != "" {
		if err := checkReadableFile(cfg.Renovate.EnvFile); err != nil {
			problems = append(problems, fmt.Sprintf("renovate.env_file: %v", err))
		}
	}

	core := client.CoreV1()
	for _, ref := range preflightRefs(cfg) {
		var err error
		switch ref.Kind {
		case "configmap":
			_, err = core.ConfigMaps(cfg.Kubernetes.Namespace).Get(ctx, ref.Name, metav1.GetOptions{})
		case "secret":
			_, err = core.Secrets(cfg.Kubernetes.Namespace).Get(ctx, ref.Name, metav1.GetOptions{})
		case "persistentvolumeclaim":
			_, err = core.PersistentVolumeClaims(cfg.Kubernetes.Namespace).Get(ctx, ref.Name, metav1.GetOptions{})
		}
		if err != nil {
			problems = append(problems, fmt.Sprintf("%s: %v", ref.Field, err))
		}
	}

	if len(problems) == 0 {
		return nil
	}
	return fmt.Errorf("preflight failed in namespace %q:\n - %s", cfg.Kubernetes.Namespace, strings.Join(problems, "\n - "))
}

// checkReadableFile verifies that path is a regular, readable file. Unlike a
// bare os.Stat, this catches directories and permission problems that would
// otherwise only surface later when loadEnvFile is called during dispatch.
func checkReadableFile(path string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()

	info, err := f.Stat()
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("%s is not a regular file", path)
	}
	return nil
}

// loadEnvFile parses a .env file (KEY=VALUE lines, blank lines and # comments
// are ignored) into a slice of corev1.EnvVar. Returns an empty slice when path
// is empty.
func loadEnvFile(path string) ([]corev1.EnvVar, error) {
	if path == "" {
		return nil, nil
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var vars []corev1.EnvVar
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, value, _ := strings.Cut(line, "=")
		vars = append(vars, corev1.EnvVar{Name: key, Value: value})
	}
	return vars, scanner.Err()
}

func buildJob(cfg *Config, repo string, envVars []corev1.EnvVar) (*batchv1.Job, error) {
	containerName := containerName(cfg.Renovate.ContainerName, repo)
	containerVolumeMounts, podVolumes, err := buildVolumeMountsAndVolumes(cfg.Renovate.VolumeMounts)
	if err != nil {
		return nil, fmt.Errorf("building volume mounts: %w", err)
	}
	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      jobName(repo),
			Namespace: cfg.Kubernetes.Namespace,
			Labels: map[string]string{
				"app.kubernetes.io/name":       "renovate",
				"app.kubernetes.io/managed-by": "renovate-scheduler",
			},
			Annotations: map[string]string{
				"renovate-scheduler/repository": repo,
			},
		},
		Spec: batchv1.JobSpec{
			BackoffLimit: new(int32(0)),
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					RestartPolicy:                corev1.RestartPolicyNever,
					ServiceAccountName:           cfg.Kubernetes.ServiceAccountName,
					AutomountServiceAccountToken: new(false),
					Volumes:                      podVolumes,
					Containers: []corev1.Container{
						{
							Name:         containerName,
							Image:        cfg.Renovate.Image,
							Args:         []string{repo},
							Env:          envVars,
							VolumeMounts: containerVolumeMounts,
							EnvFrom: []corev1.EnvFromSource{
								{
									SecretRef: &corev1.SecretEnvSource{
										LocalObjectReference: corev1.LocalObjectReference{
											Name: cfg.Kubernetes.SecretName,
										},
									},
								},
							},
							ReadinessProbe: &corev1.Probe{
								TimeoutSeconds:   5,
								PeriodSeconds:    5,
								FailureThreshold: 1,
								ProbeHandler: corev1.ProbeHandler{
									Exec: &corev1.ExecAction{
										Command: []string{"true"},
									},
								},
							},
							LivenessProbe: &corev1.Probe{
								TimeoutSeconds:   5,
								PeriodSeconds:    5,
								FailureThreshold: 1,
								ProbeHandler: corev1.ProbeHandler{
									Exec: &corev1.ExecAction{
										Command: []string{"true"},
									},
								},
							},
						},
					},
				},
			},
		},
	}

	if cfg.Scheduler.JobTimeoutSeconds > 0 {
		job.Spec.ActiveDeadlineSeconds = new(int64(cfg.Scheduler.JobTimeoutSeconds))
	}
	if cfg.Scheduler.JobTTLSeconds > 0 {
		job.Spec.TTLSecondsAfterFinished = new(int32(cfg.Scheduler.JobTTLSeconds))
	}
	return job, nil
}

func buildVolumeMountsAndVolumes(mountConfigs []RenovateVolumeMountConfig) ([]corev1.VolumeMount, []corev1.Volume, error) {
	if len(mountConfigs) == 0 {
		return nil, nil, nil
	}

	hostPathType := corev1.HostPathDirectoryOrCreate
	usedNames := map[string]int{}
	mounts := make([]corev1.VolumeMount, 0, len(mountConfigs))
	volumes := make([]corev1.Volume, 0, len(mountConfigs))

	for i, mountCfg := range mountConfigs {
		source, err := mountCfg.normalizedSource()
		if err != nil {
			return nil, nil, fmt.Errorf("volume_mounts[%d]: %w", i, err)
		}

		name := normalizeVolumeName(mountCfg.Name)
		if name == "" {
			name = fmt.Sprintf("renovate-volume-%d", i+1)
		}

		if seen := usedNames[name]; seen > 0 {
			name = fmt.Sprintf("%s-%d", name, seen+1)
		}
		usedNames[name]++

		mounts = append(mounts, corev1.VolumeMount{
			Name:      name,
			MountPath: mountCfg.MountPath,
			SubPath:   mountCfg.SubPath,
			ReadOnly:  mountCfg.ReadOnly,
		})

		volume := corev1.Volume{Name: name}
		switch source {
		case "host_path":
			volume.VolumeSource = corev1.VolumeSource{
				HostPath: &corev1.HostPathVolumeSource{
					Path: mountCfg.HostPath,
					Type: &hostPathType,
				},
			}
		case "config_map":
			volume.VolumeSource = corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{Name: mountCfg.ConfigMapName},
				},
			}
		case "secret":
			volume.VolumeSource = corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName: mountCfg.SecretName,
				},
			}
		case "persistent_volume_claim", "pvc":
			volume.VolumeSource = corev1.VolumeSource{
				PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
					ClaimName: mountCfg.PersistentVolumeClaim,
					ReadOnly:  mountCfg.ReadOnly,
				},
			}
		default:
			return nil, nil, fmt.Errorf("volume_mounts[%d]: source must be one of host_path, config_map, secret, persistent_volume_claim", i)
		}

		volumes = append(volumes, volume)
	}

	return mounts, volumes, nil
}

func normalizeVolumeName(name string) string {
	name = strings.ToLower(name)
	name = nonDNSChars.ReplaceAllString(name, "-")
	name = strings.Trim(name, "-")
	if len(name) > maxJobNameLen {
		name = strings.Trim(name[:maxJobNameLen], "-")
	}
	return name
}

var nonDNSChars = regexp.MustCompile(`[^a-z0-9-]+`)

func jobName(repo string) string {
	const prefix = "renovate-"
	suffix := "-" + randonSuffix(5)

	slug := strings.ToLower(repo)
	slug = nonDNSChars.ReplaceAllString(slug, "-")
	slug = strings.Trim(slug, "-")

	budget := maxJobNameLen - len(prefix) - len(suffix)
	if len(slug) > budget {
		slug = strings.Trim(slug[:budget], "-")
	}
	if slug == "" {
		slug = "repo"
	}
	return prefix + slug + suffix
}

func containerName(containerName string, repo string) string {
	prefix := strings.ToLower(containerName) + "-"
	suffix := "-bot-tsk-" + randonSuffix(5)

	slug := strings.ToLower(repo)
	slug = nonDNSChars.ReplaceAllString(slug, "-")
	slug = strings.Trim(slug, "-")

	budget := maxJobNameLen - len(prefix) - len(suffix)
	if len(slug) > budget {
		slug = strings.Trim(slug[:budget], "-")
	}
	if slug == "" {
		slug = "repo"
	}
	return prefix + slug + suffix
}

func randonSuffix(n int) string {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return strings.Repeat("0", n)
	}
	return hex.EncodeToString(b)[:n]
}
