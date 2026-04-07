{{/*
Expand the name of the chart.
*/}}
{{- define "snip.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Create a default fully qualified app name.
*/}}
{{- define "snip.fullname" -}}
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
Create chart label.
*/}}
{{- define "snip.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Common labels.
*/}}
{{- define "snip.labels" -}}
helm.sh/chart: {{ include "snip.chart" . }}
{{ include "snip.selectorLabels" . }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end }}

{{/*
Selector labels.
*/}}
{{- define "snip.selectorLabels" -}}
app.kubernetes.io/name: {{ include "snip.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}

{{/*
Service account name.
*/}}
{{- define "snip.serviceAccountName" -}}
{{- if .Values.serviceAccount.create }}
{{- default (include "snip.fullname" .) .Values.serviceAccount.name }}
{{- else }}
{{- default "default" .Values.serviceAccount.name }}
{{- end }}
{{- end }}

{{/*
Database URL — embedded or external.
*/}}
{{- define "snip.databaseURL" -}}
{{- if .Values.postgresql.enabled }}
{{- $host := printf "%s-postgresql" (include "snip.fullname" .) }}
{{- $user := .Values.postgresql.auth.username }}
{{- $db := .Values.postgresql.auth.database }}
{{- printf "postgres://%s:$(POSTGRES_PASSWORD)@%s:5432/%s?sslmode=disable" $user $host $db }}
{{- else }}
{{- $host := .Values.externalDatabase.host }}
{{- $port := .Values.externalDatabase.port | toString }}
{{- $user := .Values.externalDatabase.user }}
{{- $db := .Values.externalDatabase.name }}
{{- printf "postgres://%s:$(POSTGRES_PASSWORD)@%s:%s/%s?sslmode=disable" $user $host $port $db }}
{{- end }}
{{- end }}

{{/*
Redis URL — embedded or external.
*/}}
{{- define "snip.redisURL" -}}
{{- if .Values.redis.enabled }}
{{- $host := printf "%s-redis-master" (include "snip.fullname" .) }}
{{- printf "redis://%s:6379" $host }}
{{- else }}
{{- $host := .Values.externalRedis.host }}
{{- $port := .Values.externalRedis.port | toString }}
{{- printf "redis://%s:%s" $host $port }}
{{- end }}
{{- end }}

{{/*
Name of the Secret that holds the postgres password.
*/}}
{{- define "snip.databaseSecretName" -}}
{{- if .Values.postgresql.enabled }}
{{- printf "%s-postgresql" (include "snip.fullname" .) }}
{{- else if .Values.externalDatabase.existingSecret }}
{{- .Values.externalDatabase.existingSecret }}
{{- else }}
{{- printf "%s-db-credentials" (include "snip.fullname" .) }}
{{- end }}
{{- end }}

{{/*
Key inside the postgres password Secret.
*/}}
{{- define "snip.databaseSecretKey" -}}
{{- if .Values.postgresql.enabled }}
{{- "password" }}
{{- else }}
{{- "password" }}
{{- end }}
{{- end }}

{{/*
Name of the TLS Secret — derived from tls.mode.
  selfsigned / auto : managed by cert-manager, named <fullname>-tls
  manual            : user-supplied, named by tls.secretName
*/}}
{{- define "snip.tlsSecretName" -}}
{{- if eq .Values.tls.mode "manual" }}
{{- .Values.tls.secretName }}
{{- else }}
{{- printf "%s-tls" (include "snip.fullname" .) }}
{{- end }}
{{- end }}
