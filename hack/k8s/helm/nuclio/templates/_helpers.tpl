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
{{- printf "%s-controller" (include "nuclio.nuclioName" .) | trunc 63 -}}
{{- end -}}

{{- define "nuclio.scalerName" -}}
{{- printf "%s-scaler" (include "nuclio.nuclioName" .) | trunc 63 -}}
{{- end -}}

{{- define "nuclio.dlxName" -}}
{{- printf "%s-dlx" (include "nuclio.nuclioName" .) | trunc 63 -}}
{{- end -}}

{{- define "nuclio.dashboardName" -}}
{{- printf "%s-dashboard" (include "nuclio.nuclioName" .) | trunc 63 -}}
{{- end -}}

{{- define "nuclio.serviceAccountName" -}}
{{- if .Values.rbac.serviceAccountName -}}
{{- .Values.rbac.serviceAccountName -}}
{{- else -}}
{{- template "nuclio.nuclioName" . -}}
{{- end -}}
{{- end -}}

{{- define "nuclio.registryCredentialsName" -}}
{{- if .Values.registry.secretName -}}
{{- .Values.registry.secretName -}}
{{- else if .Values.registry.credentials -}}
{{- printf "%s-registry-credentials" (include "nuclio.nuclioName" .) | trunc 63 -}}
{{- else -}}
{{- printf "" -}}
{{- end -}}
{{- end -}}

{{- define "nuclio.registryPushPullUrlName" -}}
{{- printf "%s-registry-url" (include "nuclio.nuclioName" .) | trunc 63 -}}
{{- end -}}

{{- define "nuclio.functionDeployerName" -}}
{{- printf "%s-function-deployer" (include "nuclio.nuclioName" .) | trunc 63 -}}
{{- end -}}

{{- define "nuclio.crdAdminName" -}}
{{- printf "%s-crd-admin" (include "nuclio.nuclioName" .) | trunc 63 -}}
{{- end -}}

{{- define "nuclio.platformName" -}}
{{- printf "%a-platform-config" (include "nuclio.nuclioName" .) | trunc 63 -}}
{{- end -}}
