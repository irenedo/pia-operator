{{/*
Expand the name of the chart.
*/}}
{{- define "pia-operator.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Create a default fully qualified app name.
We truncate at 63 chars because some Kubernetes name fields are limited to this (by the DNS naming spec).
If release name contains chart name it will be used as a full name.
*/}}
{{- define "pia-operator.fullname" -}}
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

{{/*
Create chart name and version as used by the chart label.
*/}}
{{- define "pia-operator.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Common labels
*/}}
{{- define "pia-operator.labels" -}}
helm.sh/chart: {{ include "pia-operator.chart" . }}
{{ include "pia-operator.selectorLabels" . }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- with .Values.commonLabels }}
{{ toYaml . }}
{{- end }}
{{- end }}

{{/*
Selector labels
*/}}
{{- define "pia-operator.selectorLabels" -}}
app.kubernetes.io/name: {{ include "pia-operator.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}

{{/*
Create the name of the service account to use
*/}}
{{- define "pia-operator.serviceAccountName" -}}
{{- if .Values.serviceAccount.create }}
{{- default (include "pia-operator.fullname" .) .Values.serviceAccount.name }}
{{- else }}
{{- default "default" .Values.serviceAccount.name }}
{{- end }}
{{- end }}

{{/*
Create the name of the namespace to use
*/}}
{{- define "pia-operator.namespace" -}}
{{- if .Values.namespace.create }}
{{- default .Release.Namespace .Values.namespace.name }}
{{- else }}
{{- .Release.Namespace }}
{{- end }}
{{- end }}

{{/*
Create the args for the operator container
*/}}
{{- define "pia-operator.args" -}}
{{- $args := list }}
{{- if .Values.operator.leaderElection }}
{{- $args = append $args "--leader-elect" }}
{{- end }}
{{- if .Values.operator.aws.region }}
{{- $args = append $args (printf "--aws-region=%s" .Values.operator.aws.region) }}
{{- end }}
{{- if .Values.operator.clusterName }}
{{- $args = append $args (printf "--cluster-name=%s" .Values.operator.clusterName) }}
{{- end }}
{{- if .Values.operator.metricsBindAddress }}
{{- $args = append $args (printf "--metrics-bind-address=%s" .Values.operator.metricsBindAddress) }}
{{- end }}
{{- if .Values.operator.healthProbeBindAddress }}
{{- $args = append $args (printf "--health-probe-bind-address=%s" .Values.operator.healthProbeBindAddress) }}
{{- end }}
{{- if .Values.operator.devMode }}
{{- $args = append $args "--dev-mode" }}
{{- end }}
{{- toYaml $args }}
{{- end }}