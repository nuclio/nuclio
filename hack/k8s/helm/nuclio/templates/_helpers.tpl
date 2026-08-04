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

{{- define "nuclio.gatewayName" -}}
{{- printf "%s-gateway" (include "nuclio.fullName" .) | trunc 63 -}}
{{- end -}}

{{- define "nuclio.requireGatewayAPICRDs" -}}
{{- if not (.Capabilities.APIVersions.Has "gateway.networking.k8s.io/v1") -}}
{{- fail "Gateway API CRDs are not installed. See https://gateway-api.sigs.k8s.io/guides/getting-started" -}}
{{- end -}}
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

{{- define "nuclio.registry.credentialsSecretNames" -}}
{{- if len .Values.registry.secretNames -}}
{{- .Values.registry.secretNames | join "," -}}
{{- else -}}
{{- include "nuclio.registry.credentialsSecretName" . -}}
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

{{/*
Resolve image pull secrets for Nuclio service deployments.
Priority:
1. .Values.global.imagePullSecrets (subchart-friendly)
2. .Values.imagePullSecrets (chart-local convenience)
*/}}
{{- define "nuclio.imagePullSecrets" -}}
{{- $imagePullSecrets := .Values.global.imagePullSecrets | default .Values.imagePullSecrets | default list -}}
{{- if gt (len $imagePullSecrets) 0 -}}
imagePullSecrets:
{{- toYaml $imagePullSecrets | nindent 2 }}
{{- end }}
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
  User-supplied common labels from .Values.commonLabels.
  Safe to include even when commonLabels is empty.
*/}}
{{- define "nuclio.commonLabels" -}}
{{- if .Values.commonLabels }}
{{- toYaml .Values.commonLabels }}
{{- end -}}
{{- end -}}

{{/*
  Kubernetes recommended labels (https://kubernetes.io/docs/concepts/overview/working-with-objects/common-labels/)
  Usage: include "nuclio.standardLabels.dashboard" .  (or .controller, .dlx, .autoscaler)
  Expects . with .component set (via merge in the wrapper); has access to .Chart, .Release, etc.
  When .Values.commonLabels contains a key that collides with a generated label, the user value wins.
*/}}
{{- define "nuclio.standardLabels" -}}
{{- $instance := "" -}}
{{- if eq .component "dashboard" -}}{{- $instance = include "nuclio.dashboardName" . -}}{{- end -}}
{{- if eq .component "controller" -}}{{- $instance = include "nuclio.controllerName" . -}}{{- end -}}
{{- if eq .component "dlx" -}}{{- $instance = include "nuclio.dlxName" . -}}{{- end -}}
{{- if eq .component "autoscaler" -}}{{- $instance = include "nuclio.scalerName" . -}}{{- end -}}
{{- $generated := dict
  "app.kubernetes.io/name" .component
  "app.kubernetes.io/instance" $instance
  "app.kubernetes.io/version" (.Chart.AppVersion | toString)
  "app.kubernetes.io/component" .component
  "app.kubernetes.io/part-of" .Chart.Name
  "app.kubernetes.io/managed-by" .Release.Service
-}}
{{- $userLabels := .Values.commonLabels | default dict -}}
{{- toYaml (merge (deepCopy $userLabels) $generated) }}
{{- end -}}

{{/*
  Shortcuts: use include "nuclio.standardLabels.dashboard" . (and .controller, .dlx, .autoscaler).
  Each one adds the component name and calls the main standardLabels helper.
*/}}
{{- define "nuclio.standardLabels.dashboard" -}}{{- include "nuclio.standardLabels" (merge (dict "component" "dashboard") .) -}}{{- end -}}
{{- define "nuclio.standardLabels.controller" -}}{{- include "nuclio.standardLabels" (merge (dict "component" "controller") .) -}}{{- end -}}
{{- define "nuclio.standardLabels.dlx" -}}{{- include "nuclio.standardLabels" (merge (dict "component" "dlx") .) -}}{{- end -}}
{{- define "nuclio.standardLabels.autoscaler" -}}{{- include "nuclio.standardLabels" (merge (dict "component" "autoscaler") .) -}}{{- end -}}

{{/*
  Pod template metadata labels for a Nuclio service.
  Merges (precedence left-to-right): user-supplied podLabels, the inline
  selector-matching labels, then standardLabels (which already merges
  commonLabels with the chart's k8s-recommended labels).
  Expects . with .component set and .podLabels passed in (via merge in
  the per-component wrapper); has access to .Chart, .Release, .Values.
  When a key in podLabels collides with a chart-managed label, the user
  value wins — same precedence model as commonLabels.
*/}}
{{- define "nuclio.podTemplateLabels" -}}
{{- $component := .component -}}
{{- $compName := "" -}}
{{- if eq $component "dashboard" -}}{{- $compName = include "nuclio.dashboardName" . -}}{{- end -}}
{{- if eq $component "controller" -}}{{- $compName = include "nuclio.controllerName" . -}}{{- end -}}
{{- if eq $component "dlx" -}}{{- $compName = include "nuclio.dlxName" . -}}{{- end -}}
{{- if eq $component "autoscaler" -}}{{- $compName = include "nuclio.scalerName" . -}}{{- end -}}
{{- $selector := dict
  "app" (include "nuclio.name" .)
  "release" .Release.Name
  "nuclio.io/app" $component
  "nuclio.io/name" $compName
  "nuclio.io/class" "service"
-}}
{{- $standard := fromYaml (include "nuclio.standardLabels" .) -}}
{{- $podLabels := .podLabels | default dict -}}
{{- toYaml (merge (deepCopy $podLabels) $selector $standard) }}
{{- end -}}

{{/*
  Per-component shortcuts for nuclio.podTemplateLabels.
  Pass the component's podLabels through; helper merges with the
  generated labels giving precedence to podLabels.
*/}}
{{- define "nuclio.podTemplateLabels.dashboard" -}}{{- include "nuclio.podTemplateLabels" (merge (dict "component" "dashboard" "podLabels" .Values.dashboard.podLabels) .) -}}{{- end -}}
{{- define "nuclio.podTemplateLabels.controller" -}}{{- include "nuclio.podTemplateLabels" (merge (dict "component" "controller" "podLabels" .Values.controller.podLabels) .) -}}{{- end -}}
{{- define "nuclio.podTemplateLabels.dlx" -}}{{- include "nuclio.podTemplateLabels" (merge (dict "component" "dlx" "podLabels" .Values.dlx.podLabels) .) -}}{{- end -}}
{{- define "nuclio.podTemplateLabels.autoscaler" -}}{{- include "nuclio.podTemplateLabels" (merge (dict "component" "autoscaler" "podLabels" .Values.autoscaler.podLabels) .) -}}{{- end -}}

{{/*
  Container-level securityContext for a Nuclio service container.
  Merges the shared .Values.global.containerSecurityContext default with the
  per-component .Values.<component>.containerSecurityContext override, the
  override winning on key collisions. Renders nothing when the effective
  context is empty, so containers stay unchanged unless hardening is configured.
  Usage: include "nuclio.containerSecurityContext.dashboard" .  (or .controller, .dlx, .autoscaler)
*/}}
{{- define "nuclio.containerSecurityContext" -}}
{{- $global := dict -}}
{{- if .Values.global -}}
{{- $global = .Values.global.containerSecurityContext | default dict -}}
{{- end -}}
{{- $override := .componentSecurityContext | default dict -}}
{{- $merged := mergeOverwrite (deepCopy $global) $override -}}
{{- if $merged -}}
{{- toYaml $merged -}}
{{- end -}}
{{- end -}}

{{/*
  Per-component shortcuts for nuclio.containerSecurityContext.
  Pass the component's containerSecurityContext override through; the helper
  merges it over the shared global default.
*/}}
{{- define "nuclio.containerSecurityContext.dashboard" -}}{{- include "nuclio.containerSecurityContext" (merge (dict "componentSecurityContext" .Values.dashboard.containerSecurityContext) .) -}}{{- end -}}
{{- define "nuclio.containerSecurityContext.controller" -}}{{- include "nuclio.containerSecurityContext" (merge (dict "componentSecurityContext" .Values.controller.containerSecurityContext) .) -}}{{- end -}}
{{- define "nuclio.containerSecurityContext.dlx" -}}{{- include "nuclio.containerSecurityContext" (merge (dict "componentSecurityContext" .Values.dlx.containerSecurityContext) .) -}}{{- end -}}
{{- define "nuclio.containerSecurityContext.autoscaler" -}}{{- include "nuclio.containerSecurityContext" (merge (dict "componentSecurityContext" .Values.autoscaler.containerSecurityContext) .) -}}{{- end -}}
