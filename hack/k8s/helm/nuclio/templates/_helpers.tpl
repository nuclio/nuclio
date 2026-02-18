# Copyright 2023 The Nuclio Authors.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

{{- define "nuclio.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{- define "nuclio.fullName" -}}
{{- if .Values.fullnameOverride -}}
{{- .Values.fullnameOverride | trunc 63 | trimSuffix "-" -}}
{{- else -}}
{{- $name := (include "nuclio.name" .) -}}
{{- if contains $name .Release.Name -}}
{{- .Release.Name | trunc 63 | trimSuffix "-" -}}
{{- else -}}
{{- printf "%s-%s" .Release.Name $name | trunc 63 | trimSuffix "-" -}}
{{- end -}}
{{- end -}}
{{- end -}}

{{- define "nuclio.controllerName" -}}
{{- printf "%s-controller" (include "nuclio.fullName" .) | trunc 63 -}}
{{- end -}}

{{- define "nuclio.scalerName" -}}
{{- printf "%s-scaler" (include "nuclio.fullName" .) | trunc 63 -}}
{{- end -}}

{{- define "nuclio.dlxName" -}}
{{- printf "%s-dlx" (include "nuclio.fullName" .) | trunc 63 -}}
{{- end -}}

{{- define "nuclio.dashboardName" -}}
{{- printf "%s-dashboard" (include "nuclio.fullName" .) | trunc 63 -}}
{{- end -}}

{{- define "nuclio.serviceAccountName" -}}
{{- if .Values.rbac.serviceAccountName -}}
{{- .Values.rbac.serviceAccountName -}}
{{- else -}}
{{- template "nuclio.fullName" . -}}
{{- end -}}
{{- end -}}


{{/*
Resolve the effective docker registry url and secret Name allowing for global values
NOTE: make sure to not quote here, because an empty string is false, but a quoted string is not
*/}}
{{- define "nuclio.registry.url" -}}
{{- .Values.registry.pushPullUrl | default .Values.global.registry.url | default "" -}}
{{- end -}}

{{- define "nuclio.registry.credentialsSecretName" -}}
{{- if .Values.registry.secretName -}}
{{- .Values.registry.secretName -}}
{{- else if .Values.global.registry.secretName -}}
{{- .Values.global.registry.secretName -}}
{{- else if .Values.registry.credentials -}}
{{- printf "%s-registry-credentials" (include "nuclio.fullName" .) | trunc 63 -}}
{{- else -}}
{{- print "" -}}
{{- end -}}
{{- end -}}

{{- define "nuclio.registry.pushPullUrlName" -}}
{{- printf "%s-registry-url" (include "nuclio.fullName" .) | trunc 63 -}}
{{- end -}}

{{- define "nuclio.functionDeployerName" -}}
{{- printf "%s-function-deployer" (include "nuclio.fullName" .) | trunc 63 -}}
{{- end -}}

{{- define "nuclio.crdAdminName" -}}
{{- printf "%s-crd-admin" (include "nuclio.fullName" .) | trunc 63 -}}
{{- end -}}

{{- define "nuclio.platformConfigName" -}}
{{- printf "%s-platform-config" (include "nuclio.fullName" .) | trunc 63 -}}
{{- end -}}

{{- define "nuclio.dashboard.nodePort" -}}
{{- if .Values.dashboard.nodePort -}}
{{- .Values.dashboard.nodePort -}}
{{- else if .Values.global.nuclio.dashboard.nodePort -}}
{{- .Values.global.nuclio.dashboard.nodePort -}}
{{- else -}}
{{- print "" -}}
{{- end -}}
{{- end -}}

{{- define "nuclio.dashboard.opa.fullname" -}}
{{- if .Values.dashboard.opa.fullnameOverride -}}
{{- .Values.api.opa.fullnameOverride | trunc 63 | trimSuffix "-" -}}
{{- else -}}
{{- printf "%s-%s" (include "nuclio.dashboardName" .) .Values.dashboard.opa.name | trunc 63 | trimSuffix "-" -}}
{{- end -}}
{{- end -}}


{{- define "nuclio.externalIPAddresses" -}}
{{- if .Values.global.externalHostAddress -}}
{{- .Values.global.externalHostAddress  -}}
{{- else if len .Values.dashboard.externalIPAddresses -}}
{{- .Values.dashboard.externalIPAddresses | join "," | quote -}}
{{- else -}}
# leave empty if no input were given.
# we resolve external ip address via `kubectl get nodes` or via the kubeconfig host
{{- "" -}}
{{- end -}}
{{- end -}}

{{/*
  Kubernetes recommended labels (https://kubernetes.io/docs/concepts/overview/working-with-objects/common-labels/)
  Usage: include "nuclio.standardLabels.dashboard" .  (or .controller, .dlx, .autoscaler)
  Expects . with .component set (via merge in the wrapper); has access to .Chart, .Release, etc.
*/}}
{{- define "nuclio.standardLabels" -}}
{{- $instance := "" -}}
{{- if eq .component "dashboard" -}}{{- $instance = include "nuclio.dashboardName" . -}}{{- end -}}
{{- if eq .component "controller" -}}{{- $instance = include "nuclio.controllerName" . -}}{{- end -}}
{{- if eq .component "dlx" -}}{{- $instance = include "nuclio.dlxName" . -}}{{- end -}}
{{- if eq .component "autoscaler" -}}{{- $instance = include "nuclio.scalerName" . -}}{{- end -}}
app.kubernetes.io/name: {{ .component }}
app.kubernetes.io/instance: {{ $instance | quote }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
app.kubernetes.io/component: {{ .component }}
app.kubernetes.io/part-of: {{ .Chart.Name }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end -}}

{{/*
  Shortcuts: use include "nuclio.standardLabels.dashboard" . (and .controller, .dlx, .autoscaler).
  Each one adds the component name and calls the main standardLabels helper.
*/}}
{{- define "nuclio.standardLabels.dashboard" -}}{{- include "nuclio.standardLabels" (merge (dict "component" "dashboard") .) -}}{{- end -}}
{{- define "nuclio.standardLabels.controller" -}}{{- include "nuclio.standardLabels" (merge (dict "component" "controller") .) -}}{{- end -}}
{{- define "nuclio.standardLabels.dlx" -}}{{- include "nuclio.standardLabels" (merge (dict "component" "dlx") .) -}}{{- end -}}
{{- define "nuclio.standardLabels.autoscaler" -}}{{- include "nuclio.standardLabels" (merge (dict "component" "autoscaler") .) -}}{{- end -}}
