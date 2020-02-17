# Copyright 2017 The Nuclio Authors.
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

{{- define "nuclio.nuclioName" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{- define "nuclio.controllerName" -}}
{{- printf "%s-controller" .Release.Name | trunc 63 -}}
{{- end -}}

{{- define "nuclio.scalerName" -}}
{{- printf "%s-scaler" .Release.Name | trunc 63 -}}
{{- end -}}

{{- define "nuclio.dlxName" -}}
{{- printf "%s-dlx" .Release.Name | trunc 63 -}}
{{- end -}}

{{- define "nuclio.dashboardName" -}}
{{- printf "%s-dashboard" .Release.Name | trunc 63 -}}
{{- end -}}

{{- define "nuclio.serviceAccountName" -}}
{{- if .Values.rbac.serviceAccountName -}}
{{- .Values.rbac.serviceAccountName -}}
{{- else -}}
{{- printf "%s-nuclio" .Release.Name -}}
{{- end -}}
{{- end -}}

{{- define "nuclio.registryCredentialsName" -}}
{{- if .Values.registry.secretName -}}
{{- .Values.registry.secretName -}}
{{- else if .Values.registry.credentials -}}
{{- printf "%s-registry-credentials" .Release.Name -}}
{{- else -}}
{{- printf "" -}}
{{- end -}}
{{- end -}}

{{- define "nuclio.registryPushPullUrlName" -}}
{{- printf "%s-registry-url" .Release.Name -}}
{{- end -}}

{{- define "nuclio.functionDeployerName" -}}
{{- printf "%s-function-deployer" .Release.Name -}}
{{- end -}}

{{- define "nuclio.crdAdminName" -}}
{{- printf "%s-crd-admin" .Release.Name -}}
{{- end -}}

{{- define "nuclio.platformName" -}}
{{- printf "platform-config" -}}
{{- end -}}
