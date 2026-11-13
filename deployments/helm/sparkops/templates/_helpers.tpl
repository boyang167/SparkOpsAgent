{{/*
Expand the name of the chart.
*/}}
{{- define "sparkops.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Create a default fully qualified app name.
*/}}
{{- define "sparkops.fullname" -}}
{{- if .Values.fullnameOverride }}
{{- .Values.fullnameOverride | trunc 63 | trimSuffix "-" }}
{{- else }}
{{- $name := default .Chart.Name .Values.nameOverride }}
{{- if contains $name .Release.Name }}
{{- .Release.Name | trunc 63 | trimSuffix "-" }}
{{- else }}
{{- printf "%s-%s" .Release.Name $name | trunc 63 | trimSuffix "-" }}
{{- end }}
{{- end }}
{{- end }}

{{- define "sparkops.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}

{{- define "sparkops.labels" -}}
helm.sh/chart: {{ include "sparkops.chart" . }}
{{ include "sparkops.selectorLabels" . }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end }}

{{- define "sparkops.selectorLabels" -}}
app.kubernetes.io/name: {{ include "sparkops.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}

{{- define "sparkops.api.selectorLabels" -}}
{{ include "sparkops.selectorLabels" . }}
app.kubernetes.io/component: api
{{- end }}

{{- define "sparkops.web.selectorLabels" -}}
{{ include "sparkops.selectorLabels" . }}
app.kubernetes.io/component: web
{{- end }}

{{- define "sparkops.serviceAccountName" -}}
{{- if .Values.serviceAccount.create }}
{{- default (include "sparkops.fullname" .) .Values.serviceAccount.name }}
{{- else }}
{{- default "default" .Values.serviceAccount.name }}
{{- end }}
{{- end }}

{{- define "sparkops.api.fullname" -}}
{{- printf "%s-api" (include "sparkops.fullname" .) }}
{{- end }}

{{- define "sparkops.web.fullname" -}}
{{- printf "%s-web" (include "sparkops.fullname" .) }}
{{- end }}

{{- define "sparkops.filesystem.fullname" -}}
{{- printf "%s-%s" (include "sparkops.fullname" .root) .component | trunc 63 | trimSuffix "-" }}
{{- end }}

{{- define "sparkops.filesystem.selectorLabels" -}}
{{ include "sparkops.selectorLabels" .root }}
app.kubernetes.io/component: {{ .component }}
{{- end }}

{{- define "sparkops.filesystem.secretName" -}}
{{- default (printf "%s-secrets" (include "sparkops.fullname" .)) .Values.filesystem.existingSecret }}
{{- end }}
