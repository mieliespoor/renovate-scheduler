{{- define "renovate-scheduler.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{- define "renovate-scheduler.fullname" -}}
{{- if .Values.fullnameOverride -}}
{{- .Values.fullnameOverride | trunc 63 | trimSuffix "-" -}}
{{- else -}}
{{- printf "%s-%s" .Release.Name (include "renovate-scheduler.name" .) | trunc 63 | trimSuffix "-" -}}
{{- end -}}
{{- end -}}

{{- define "renovate-scheduler.labels" -}}
app.kubernetes.io/name: {{ include "renovate-scheduler.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
helm.sh/chart: {{ printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" }}
{{- end -}}

{{- define "renovate-scheduler.serviceAccountName" -}}
{{- default (include "renovate-scheduler.fullname" .) .Values.serviceAccount.name -}}
{{- end -}}

{{- define "renovate-scheduler.jobServiceAccountName" -}}
{{- .Values.scheduler.config.kubernetes.serviceAccountName -}}
{{- end -}}

{{- define "renovate-scheduler.secretName" -}}
{{- if .Values.scheduler.existingSecret -}}
{{- .Values.scheduler.existingSecret -}}
{{- else -}}
{{- printf "%s-secret" (include "renovate-scheduler.fullname" .) -}}
{{- end -}}
{{- end -}}

{{- define "renovate-scheduler.createSecret" -}}
{{- if not .Values.scheduler.existingSecret -}}true{{- end -}}
{{- end -}}

{{- define "renovate-scheduler.configMapName" -}}
{{- default (include "renovate-scheduler.fullname" .) .Values.scheduler.existingConfigMap -}}
{{- end -}}

{{- define "renovate-scheduler.reposConfigMapName" -}}
{{- default (printf "%s-repos" (include "renovate-scheduler.fullname" .)) .Values.scheduler.existingReposConfigMap -}}
{{- end -}}

{{- define "renovate-scheduler.pvcName" -}}
{{- default (printf "%s-state" (include "renovate-scheduler.fullname" .)) .Values.persistence.existingClaim -}}
{{- end -}}

{{- define "renovate-scheduler.validateValues" -}}
{{- if .Values.scheduler.config.kubernetes.secretName -}}
{{- fail "scheduler.config.kubernetes.secretName is no longer supported; use scheduler.existingSecret." -}}
{{- end -}}
{{- end -}}
