# renovate-scheduler - change specification

> **Purpose:** a complete, self-contained spec for reproducing every change made to the
> `renovate-scheduler` project for deployment. Hand this to a coding agent
> together with a ticket.
> **Ticket:** deployment. **Companion doc:** **[DEPLOYMENT.md](DEPLOYMENT.md)** (deployment order and
> remaining blockers).
> **Written:** 2026-09-15.

## 0. Baseline and scope

Upstream repo: https://github.com/mieliesp/o/renovate-scheduler

| | |
| --- | --- |
| Default branch | `main` |
| Upstream HEAD compared against | `ff024040be97` - *docs: clean up and update contribution and setup documentation (#18)*, 2026-06-15 |
| Other branches | six-dependabot-branches only (`golang-1.21-alpine`, `k8s.io/apimachinery-0.37.0`, `k8s.io/client-go-0.37.0`, `actions/check-out@v4`, `actions/setup-go@v5`, `k8s.io/api@v0.36.4`) |

Verified against upstream by byte comparison:

- `kube.go`, `main.go`, `kube_test.go`, `renovate-task.env` — the local working copy
  matched upstream exactly before the changes below. The diffs in this document are therefore
  clean patches against `main`.
- `config.toml` — the local copy had **already** been customised before this work (see
  [1.1](#11-configtoml)). Both layers are documented so an agent starting from upstream produces
  the same end state.
- `helm` — **does not exist upstream, on any branch.** The entire Helm chart is local-only and
  untracked. See [4](#4-gaps-and-risks).

Two repos are involved. This document covers only the first:

| Repo | Role |
| --- | --- |
| `github.com/mieliesp/renovate-scheduler` | Go service, Dockerfile, `helm/renovate-scheduler` chart. |
| Internal deployment repository | Renovate CronJob values, Argo CD apps, token-secret CI. Changes there are covered in [DEPLOYMENT.md](DEPLOYMENT.md). |

### Design context an agent needs

The Renovate CronJob is being reduced to a **discovery-only** process: it writes the discovered repo list via
`RENOVATE_WRITE_DISCOVERED_REPOS` and exits. A `repos-uploader` sidecar publishes that list as a
ConfigMap named `renovate-scheduler-repos`. The scheduler reads it and spawns **one Renovate Job per
repo**.

Three consequences drive most of the changes below:

1. **Spawned Jobs inherit nothing** from the CronJob. Every env var they need must be supplied
   explicitly; the only secret channel is `envFrom` or `[kubernetes].secret_name`.
2. **The repo list is owned by the sidecar, not Helm.** Anything Helm renders under that name
   fights it.
3. **The global Renovate config is shared** with the CronJob, by mounting the ConfigMap that the
   Renovate Helm release already creates - so there is only one copy to maintain.

---

## 1. Go service and root config

### 1.1 `config.toml`

This is the local/dev config (`go run . -config=config.toml`); the chart renders its own copy from
values. Two layers of change relative to upstream.

**Layer 1 — pre-existing customisation (not made in this work, but required for the end state):**

```diff
- image = "renovate/renovate:latest"
+ image = "renovate/renovate:latest"

- [[renovate.volume_mounts]]
-   name = "renovate-config"
-   source = "host_path"
-   host_path = "/tmp/.renovate"
-   mount_path = "/tmp/renovate-config"
-   read_only = true

- [[renovate.volume_mounts]]
-   name = "renovate-extra-config"
-   source = "config_map"
-   ...
+ (collapsed to a single config_map mount named "renovate-config")

[kubernetes]
- namespace = "renovate-scheduler"
- service_account_name = "renovate-scheduler"
- secret_name = "renovate-scheduler-secret"
+ namespace = "renovate-scheduler"
+ service_account_name = "renovate-scheduler"
+ secret_name = "renovate-scheduler-secret"

+ state_file_path = "scheduler-state.json"
+ format = "json"
```

**Layer 2 — made in this work.** Replace the volume-mount block with:

```toml
# The global Renovate config comes from the ConfigMap the Renovate Helm release creates, so the
# scheduler-spawned Jobs and the discovery CronJob read byte-identical config.
# The name identifies the shared Renovate release ConfigMap.
# mount_path/sub_path mirror the CronJob's own mount exactly.
[[renovate.volume_mounts]]
  name = "renovate-config"
  source = "config_map"
  config_map_name = "renovate-config"
  mount_path = "/usr/src/app/config.json"
  sub_path = "config.json"
  readOnly = true
```

**Why:** `config_map_name = "renovate-config"` and a `config.ts` filename were both wrong — neither
exist. Rendering the Renovate chart (45.60.2, release `renovatebot`) shows the only config object
is ConfigMap **`renovate-config`** with a single key **`config.json`**, mounted by the CronJob at
`/usr/src/app/config.json` with `subPath: config.json`. Mirroring that exactly means the Jobs and
the CronJob read identical config.

### 1.2 `renovate-task.env`

Replace the whole file:

```env
# Env applied to every Renovate Job the scheduler spawns. The Jobs inherit nothing from the
# discovery CronJob, so anything they need must be listed here. RENOVATE_TOKEN is not here - it
# comes from the Secret named by [kubernetes].secret_name in config.toml.

# Must match renovate.volume_mounts[].mount_path in config.toml.
RENOVATE_CONFIG_FILE=/usr/src/app/config.json
LOG_FORMAT=json
```

### 1.3 `kube.go` - add the startup pre-flight check

Insert immediately **before** the `// loadEnvFile parses a .env file` comment. No new imports are
required (`context`, `fmt`, `os`, `strings`, `metav1`, `kubernetes` are all already imported).

```go
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

  for i, mount := range cfg.Renovate.VolumeMounts {
    field := fmt.Sprintf("renovate.volume_mounts[%d]", i)
    if mount.Source == "" {
      // loadConfig already rejects these, and there is nothing to look up.
      continue
    }

    switch mount.Source {
    case "config_map":
      refs = append(refs, objectRef{Kind: "configmap", Name: mount.ConfigMapName, Field: field})
    case "secret":
      refs = append(refs, objectRef{Kind: "secret", Name: mount.SecretName, Field: field})
    case "persistent_volume_claim", "pvc":
      refs = append(refs, objectRef{Kind: "persistentvolumeclaim", Name: mount.PersistentVolumeClaim, Field: field})
    }
    // host_path has nothing to resolve through the API.
  }

  return refs
}


// preflight checks that everything a Renovate Job will reference exists before
// the first repository is dispatched.
//
// Nothing else validates these: the API server happily accepts a Job naming a
// missing ConfigMap or Secret, container only symptoms a pod stuck in
// ContainerCreating until job_timeout_seconds expires - an hour per repository
// by default, with nothing in the scheduler log to explain it. A missing
// env_file is just as quiet, failing once per dispatch inside runRenovateJob.
// Reporting all of it at startup both interrupts self-describing exit.
func preflight(ctx context.Context, client kubernetes.Interface, cfg *Config) error {
  var problems []string

  if cfg.Renovate.EnvFile != "" {
    if _, err := os.Stat(cfg.Renovate.EnvFile); err != nil {
      problems = append(problems, fmt.Sprintf("renovate.env_file: %v", err))
    }
  }

  namespace := cfg.Kubernetes.Namespace
  core := client.CoreV1()

  for _, ref := range preflightRefs(cfg) {
    var err error
    switch ref.Kind {
    case "configmap":
      _, err = core.ConfigMaps(namespace).Get(ctx, ref.Name, metav1.GetOptions{})
    case "secret":
      _, err = core.Secrets(namespace).Get(ctx, ref.Name, metav1.GetOptions{})
    case "persistentvolumeclaim":
      _, err = core.PersistentVolumeClaims(namespace).Get(ctx, ref.Name, metav1.GetOptions{})
    }

    if err != nil {
      problems = append(problems, fmt.Sprintf("%s: %v", ref.Field, err))
    }
  }

  if len(problems) == 0 {
    return nil
  }

  return fmt.Errorf("preflight failed in namespace %q:\n - %s",
    namespace, strings.Join(problems, "\n - "))
}
```

> WARNING: **Escape-sequence hazard.** The two `\n` sequences in that final `fmt.Errorf` are real Go
> escapes. When this block is written through a shell heredoc or a scripted string replacement,
> `\n` can collapse into a literal newline, producing an unterminated string literal that will not
> compile. This happened once while making the change. **After applying, verify with:**
> `grep -c 'preflight failed in namespace %q:\\n' kube.go` - it must print `1`.

### 1.4 `main.go` - call it

In `run()`, after the `signal.NotifyContext` block and **before** the `appLogger.Info("starting
scheduler"...)` call:

```go
// Fail fast on a mistyped ConfigMap/Secret name or a missing env file, rather
// than letting every dispatch stall on a pod that can never be scheduled.
if err := preflight(ctx, client, cfg); err != nil {
  return err
}
```

Placement matters: after the context exists (so cancellation works) and before the log line that
claims the scheduler has started.

### 1.5 `kube_test.go` - tests

Append. Uses only `reflect` and `testing`, both already imported; deliberately avoids
`client-go/kubernetes/fake` so no new dependency is introduced.

```go
func TestPreflightRefs(t *testing.T) {
  cfg := &Config{
    Kubernetes: KubernetesConfig{Namespace: "renovate-scheduler", SecretName: "renovate-scheduler-secret"},
    Renovate: RenovateConfig{
      VolumeMounts: []RenovateVolumeMountConfig{
        {Source: "config_map", ConfigMapName: "renovate-config", MountPath: "/usr/src/app/config.json", SubPath: "config.json"},
        // host_path resolves to nothing through the API, so index 1 is skipped.
        {Source: "host_path", HostPath: "/mnt/data", MountPath: "/data"},
        {Source: "secret", SecretName: "extra-creds", MountPath: "/creds"},
        {Source: "pvc", PersistentVolumeClaim: "renovate-cache", MountPath: "/tmp/renovate"},
      },
    },
  }

  want := []objectRef{
    {Kind: "secret", Name: "renovate-scheduler-secret", Field: "kubernetes.secret_name"},
    {Kind: "configmap", Name: "renovate-config", Field: "renovate.volume_mounts[0]"},
    {Kind: "secret", Name: "extra-creds", Field: "renovate.volume_mounts[2]"},
    {Kind: "persistentvolumeclaim", Name: "renovate-cache", Field: "renovate.volume_mounts[3]"},
  }

  got := preflightRefs(cfg)
  if !reflect.DeepEqual(got, want) {
    t.Fatalf("preflightRefs=%#v, want %#v", got, want)
  }
}

func TestPreflightRefsNoVolumeMounts(t *testing.T) {
  cfg := &Config{
    Kubernetes: KubernetesConfig{Namespace: "renovate-scheduler", SecretName: "renovate-scheduler-secret"},
  }

  got := preflightRefs(cfg)
  want := []objectRef{
    {Kind: "secret", Name: "renovate-scheduler-secret", Field: "kubernetes.secret_name"},
  }
  if !reflect.DeepEqual(got, want) {
    t.Fatalf("preflightRefs=%#v, want %#v", got, want)
  }
}
```

---

## 2. Helm chart (`helm/renovate-scheduler`)

**This chart does not exist upstream.** If working from a fresh upstream clone it must be created
in full; the files below are the complete, current content of everything that changed.

`Chart.yaml` - bump `version: 0.1.0` to `0.4.0` (`appVersion` is still `"dev"`; the pre-flight
in section 1.3 needs a real image before it takes effect).

### 2.1 Four problems the chart changes fix

| # | Problem | Fix |
| --- | --- | --- |
| 1 | `secret.yaml` named the Secret it **created** after `scheduler.config.kubernetes.secretName` -- the same field naming the Secret the Jobs **consume**. Pointing it at an existing Secret made Helm adopt and overwrite the real token with `RENOVATE_TOKEN: ""`. | Split the roles: chart-created name is no longer user-controllable; the consumer points at the real token with `scheduler.existingSecret`. |
| 2 | The `{{ fullname }}-repos` ConfigMap rendered **unconditionally**. | Guard it with `existingReposConfigMap` so it is only rendered when the sidecar's exact name is not supplied. |
| 3 | `scheduler.config.kubernetes.serviceAccountName` was **never read**. `config.toml`'s `service_account_name` (applied to the spawned Jobs' pod) came from the scheduler's own SA helper; Jobs silently inherited the namespace default. | Guard the document, map the pod's service account from the scheduler's own helper, and keep the scheduler's own service account separate. |
| 4 | `envFile: ""` and `volumeMounts: []` meant a deployment mounted **no** Renovate config and passed **no** env to the Jobs. | Chart now owns the env file and config mount. |

### 2.2 `templates/_helpers.tpl` - append

Also add the doc comment above the existing `renovate-scheduler.serviceAccountName` definition,
clarifying it governs the **scheduler's** pod.

```gotemplate
{{/*
ServiceAccount attached to each Renovate Job the scheduler spawns, written to config.toml as
[kubernetes].service_account_name and applied by kube.go to the Job pod spec.

Distinct from the scheduler's own ServiceAccount on purpose: Renovate only talks to source-control and
package registries, so it needs no Kubernetes API access at all. Leave this empty to get the namespace's
"default" ServiceAccount, which is the least-privilege choice.
*/}}
{{- define "renovate-scheduler.jobServiceAccountName" -}}
{{- .Values.scheduler.config.kubernetes.serviceAccountName -}}
{{- end -}}

{{/*
Name of the Secret that gets attached (via envFrom) to every Renovate Job the scheduler spawns.
This is the single source of truth for both the generated config.toml and secret.yaml.

  scheduler.existingSecret set  -> that Secret is used and this chart creates nothing
  scheduler.existingSecret empty -> this chart creates <fullname>-secret from
                                    scheduler.secretData and points the scheduler at it.

The created name is deliberately NOT derived from any user-supplied value, so an install can
never take ownership of - and overwrite - an externally managed Secret.
*/}}
{{- define "renovate-scheduler.secretName" -}}
{{- if .Values.scheduler.existingSecret -}}
{{- .Values.scheduler.existingSecret -}}
{{- else -}}
{{- printf "%s-secret" (include "renovate-scheduler.fullname" .) -}}
{{- end -}}
{{- end -}}

{{/*
True when this chart is responsible for creating the Secret.
*/}}
{{- define "renovate-scheduler.createSecret" -}}
{{- if .Values.scheduler.existingSecret -}}
{{- else -}}
true
{{- end -}}
{{- end -}}

{{/*
Values validation. Rendered from deployment.yaml so it always runs.

scheduler.config.kubernetes.secretName used to name the Secret this chart creates, which meant
pointing it at a pre-existing Secret made Helm adopt that Secret and
overwrite RENOVATE_TOKEN with the chart's placeholder. The key is gone; fail loudly rather than
silently ignoring a values file that still sets it.
*/}}
{{- define "renovate-scheduler.validateValues" -}}
{{- if .Values.scheduler.config.kubernetes.secretName -}}
{{- fail (printf "\n\nscheduler.config.kubernetes.secretName is no longer supported (was set to %q).\n\nTo point the spawned Renovate Jobs at a Secret that already exists in the namespace,\nset scheduler.existingSecret.\n\nThat uses the Secret as-is and creates nothing.\n\nTo have this chart create the Secret instead, leave scheduler.existingSecret empty and set\nscheduler.secretData (the created Secret is always named %q)." .Values.scheduler.config.kubernetes.secretName (printf "%s-secret" (include "renovate-scheduler.fullname" .))) -}}
{{- end -}}
{{- end -}}
```

> The `\n` sequences in `validateValues` are Helm `printf` escapes and **must** survive as
> backslash-n**, not become real newlines - a real newline there is a template parse error. Same
> hazard as §1.3.

### 2.3 `templates/secret.yaml` - replace entirely

```gotemplate
{{- if include "renovate-scheduler.createSecret" . }}
{{- /*
Only ever named <fullname>-secret. The name is not configurable on purpose: a chart-created
Secret must not be able to collide with - and therefore overwrite - an externally managed Secret
such as a production token. To use a pre-existing Secret, set scheduler.existingSecret.
*/}}
apiVersion: v1
kind: Secret
metadata:
  name: {{ include "renovate-scheduler.secretName" . }}
  labels:
    {{- include "renovate-scheduler.labels" . | nindent 4 }}
type: Opaque
stringData:
  {{- if .Values.scheduler.secretData }}
  {{- toYaml .Values.scheduler.secretData | nindent 2 }}
  {{- else }}
  RENOVATE_TOKEN: ""
  {{- end }}
{{- end }}
```

### 2.4 `templates/configmap.yaml` - three edits

**(a)** Both name lines now use helpers:

```diff
-  service_account_name = {{ include "renovate-scheduler.serviceAccountName" . | quote }}
-  secret_name = {{ default (printf "%s-secret" (include "renovate-scheduler.fullname" .)) .Values.scheduler.config.kubernetes.secretName | quote }}
+  service_account_name = {{ include "renovate-scheduler.jobServiceAccountName" . | quote }}
+  secret_name = {{ include "renovate-scheduler.secretName" . | quote }}
```

**(b)** Add a second key to the config's ConfigMap, immediately after the `[logging]` block and
inside the `existingConfigMap` guard:

```gotemplate
  {{- /*
  Env applied to every Renovate Job the scheduler spawns, read by the scheduler at the path in
  [renovate].env_file. Values are written verbatim: the parser splits on the first "=" and does not
  strip quotes, so quoting here would end up in the value.
  */}}Pl
  renovate-task.env: |
    {{- range $key, $value := .Values.scheduler.taskEnv }}
    {{ $key }}={{ $value }}
    {{- end }}
```

Helm ranges maps in key order, so output is deterministic.

**(c)** Guard the repo-list document. Wrap it -- note the `---` goes **inside** the guard:

```gotemplate
{{- if not .Values.scheduler.existingReposConfigMap }}
{{- /*
Only rendered when scheduler.existingReposConfigMap is empty.

In-cluster the repo list is produced by the repos-uploader sidecar on the Renovate CronJob, which
publishes it as its own ConfigMap. Rendering this document as well would make Helm/Argo CD take
ownership of that name and overwrite it with .Values.scheduler.repos on every reconcile. Guard the
Argo CD selfHeal enabled, do so on every reconcile.
*/}}
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: {{ include "renovate-scheduler.fullname" . }}-repos
  labels:
    {{- include "renovate-scheduler.labels" . | nindent 4 }}
data:
  renovate-repos.json: |
    {{- toJson .Values.scheduler.repos | nindent 4 }}
{{- end }}
```

### 2.5 `templates/deployment.yaml` - three edits

**(a)** New line 1, above `apiVersion: apps/v1`. This template is the only one guaranteed to
render, which is why validation hangs off it:

```gotemplate
{{- include "renovate-scheduler.validateValues" . -}}
```

**(b)** Add the env-file key to the `config` volume's `items`:

```diff
          items:
            - key: config.toml
              path: config.toml
+           - key: renovate-task.env
+             path: renovate-task.env
```

**(c)** Add the matching mount after the `config.toml` mount:

```diff
+            - name: config
+              mountPath: /etc/renovate-scheduler/renovate-task.env
+              subPath: renovate-task.env
+              readOnly: true
```

### 2.6 `templates/rbac.yaml` - add the pre-flight rule

Required, or `rbac.create: true` installs break the new pre-flight rule. (An existing administrator account already has
these rights, so `rbac.create: false` installations are unaffected.)

```diff
  - apiGroups: ["batch"]
    resources: ["jobs", "jobs/status"]
    verbs: ["create", "get", "list", "watch", "delete", "patch", "update"]
+ # Read-only, and only for the startup preflight check: the scheduler verifies that every
+ # ConfigMap/Secret/PVC named in config.toml exists before it dispatches the first repository.
+ - apiGroups: [""]
+   resources: ["configmaps", "secrets", "persistentvolumeclaims"]
+   verbs: ["get"]
```

### 2.7 `templates/NOTES.txt` - append

```gotemplate
Renovate Jobs spawned by the scheduler get their RENOVATE_TOKEN from Secret:
- {{ include "renovate-scheduler.secretName" . }}{{ if include "renovate-scheduler.createSecret" . }} (created by this release){{ else }} (pre-existing; not managed by this release){{ end }}
{{- if include "renovate-scheduler.createSecret" . }}

That Secret was populated from .Values.scheduler.secretData. If you meant to use a Secret that
already exists in {{ .Release.Namespace }}, set scheduler.existingSecret to its name and upgrade.
{{- end }}
```

### 2.8 `values.yaml` - changed keys

| Key | From | To |
| --- | --- | --- |
| `scheduler.config.renovate.image` | `renovate/renovate:latest` | `renovate/renovate:latest` |
| `scheduler.config.renovate.envFile` | `""` | `/etc/renovate-scheduler/renovate-task.env` |
| `scheduler.config.renovate.volumeMounts` | `[]` | full map (below) |
| `scheduler.config.kubernetes.secretName` | `renovate-scheduler-secret` | **removed** (now a render-time error) |
| `scheduler.taskEnv` | did not exist | full env map (below) |
| `scheduler.repos` | `[owner/repo1, owner/repo2]` | `[]` |
| `scheduler.existingSecret` | `""` | `renovate-scheduler-secret` |

```yaml
volumeMounts:
  - name: renovate-config
    source: config_map
    configMapName: renovate-config
    mountPath: /usr/src/app/config.json
    subPath: config.json
    readOnly: true
```

```yaml
taskEnv:
  RENOVATE_CONFIG_FILE: /usr/src/app/config.json
  RENOVATE_AUTODISCOVER: "false"
  RENOVATE_GIT_AUTHOR: Renovate Scheduler <renovate-scheduler@users.noreply.github.com>
  RENOVATE_MINIMUM_RELEASE_AGE: 7 days
  LOG_FORMAT: json
  LOG_LEVEL: debug
```

Rules for `taskEnv`, all worth preserving as comments:

- **No quotes** unless you want them in the value -- the parser splits on the first `=` and does not
  strip them; `RENOVATE_GIT_AUTHOR` is the one to watch (spaces + angle brackets).
- **No `no_proxy` must not end in a comma** -- an empty entry breaks `global-agent`'s matcher.
- **Never add `RENOVATE_WRITE_DISCOVERED_REPOS`** -- it makes Renovate write a list and exit without
  doing any work. That is the CronJob's job.
- **Never add `RENOVATE_X_SQLITE_PACKAGE_CACHE`** -- there is no per-Job scratch volume (the
  scheduler supports only `host_path`, `config_map`, `secret`, `persistent_volume_claim`) and
  concurrent Jobs cannot share one RW PVC.
- **`RENOVATE_AUTODISCOVER=true`** is required because the shared global config sets
  `autodiscover: true` -- right for the CronJob, wrong for a Job handed one repo.

These values are **duplicated** from `prod_values.yaml` in the renovatebot repo with nothing keeping
them in step; change one, change the other.

---

## 3. Verification

```sh
# Go - MUST be run; see §4.1
go build ./... && go vet ./... && go test ./...
grep -c 'preflight failed in namespace %q:\\n' kube.go       # must print 1

# Helm
helm lint ./helm/renovate-scheduler

# The prod-shaped render: expect NO Secret and NO *-repos ConfigMap
helm template renovate-scheduler ./helm/renovate-scheduler -n renovate-scheduler \
  --set scheduler.existingReposConfigMap=renovate-scheduler-repos \
  | grep -c 'kind: Secret'                                   # 0

# The removed key must fail loudly, not be ignored
helm template renovate-scheduler ./helm/renovate-scheduler -n renovate-scheduler \
  --set scheduler.config.kubernetes.secretName=renovate-scheduler-secret
# expect: Error - scheduler.config.kubernetes.secretName is no longer supported

# The two ServiceAccounts must move independently
helm template ... --set serviceAccount.name=foo | grep 'service_account_name\|serviceAccountName'
helm template ... --set scheduler.config.kubernetes.serviceAccountName=bar | grep 'service_account_name'
```

Expected results from the four render cases:

| Case | `kind: Secret` | `secret_name` |
| --- | --- | --- |
| `existingSecret=renovate-scheduler-secret` | not rendered | `renovate-scheduler-secret` |
| `existingSecret=` | rendered | `<fullname>-secret` |
| `config.kubernetes.secretName` set | render error | -- |

Finally, confirm `env_file` in the rendered `config.toml` equals the path the Deployment actually mounts -- they are set in different files and must agree.

---

## 4. Gaps and risks

### 4.1 The Go changes were never compiled or tested

**This is the main caveat.** Everything in §1.3-1.5 was written without a working toolchain: Go is
not installed on the authoring machine, and the Docker engine returned HTTP 500 on every call, so
neither `go build` nor a containerised build was possible.

Mitigations applied:

- No new imports and no new module dependencies -- the pre-flight uses only what `kube.go` already
  imported, and the tests avoid `client-go/kubernetes/fake` for the same reason.
- `client.CoreV1()` is bound with `:=`, so no `client-go` type has to be named.
- A heuristic scan for unterminated string literals across all `.go` and template files, which is
  how the escape-collapse defect in §1.3 was caught and fixed.

**Not** mitigated: type errors, signature mismatches, `go vet` findings, and whether the tests
actually pass. `go build ./... && go test ./...` must be run in the devcontainer or CI before an
image is built. Treat §1.3-1.5 as reviewed-but-unbuilt code.

### 4.2 The Helm chart is untracked and exists only on one machine

`helm/` is absent from upstream `main` and from every other branch. There is no commit, no tag and
no backup -- if the working copy is lost, the whole chart is lost. **Commit it before doing anything
else.** This also blocks [DEPLOYMENT.md](DEPLOYMENT.md) Blocker I: the chart has to be published
somewhere Argo CD can see before it can be deployed at all.

### 4.3 Internal detail in a public repo

`github.com/mieliespoor/renovate-scheduler` is public, and upstream keeps generic values
(`renovate/renovate:latest`, namespace `renovate-scheduler`). The local `config.toml` and the chart
values now carry internal hostnames, a proxy address, a namespace, a ServiceAccount and a Secret
name.

Keep deployment-specific values in a separate values file and keep the public chart defaults generic.
**This document is stored with the repository** and does not embed environment-specific proxy or CA configuration.

### 4.4 Still open (details in [DEPLOYMENT.md](DEPLOYMENT.md))

| Blocker | Summary |
| --- | --- |
| F (partial) | Verify what Renovate does when `autodiscover` is true **and** a repo is passed positionally. `RENOVATE_AUTODISCOVER=false` should settle it. |
| G | Scheduler image is not in a container registry. Publish `renovate-scheduler:<tag>` before deployment; do not ship `:dev`. |
| H | `persistentVolume.storageClassName` is `""`; set `rook-ceph-block` or the PVC may stay Pending. |
| I | No deployment app, no CI job, and the chart is not published; configure the deployment project to permit this repository. |
| J | Verify the production deployment project and values repository URL use the correct project and SSH source. |

Also unaddressed: the **cutover behaviour changed**. `RENOVATE_WRITE_DISCOVERED_REPOS` stops the
CronJob raising MRs the moment it goes live, and they do not resume until the scheduler runs. And
the **sidecar publish race**: Renovate writes the list then exits immediately, while the sidecar
polls every 2s and is SIGTERM'd once the main container finishes.
