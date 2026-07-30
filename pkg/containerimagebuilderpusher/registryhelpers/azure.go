/*
Copyright 2026 The Nuclio Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package registryhelpers

import (
	"fmt"
	"strings"

	"github.com/nuclio/nuclio/pkg/common"

	v1 "k8s.io/api/core/v1"
)

// acrLoginUsername is the fixed GUID for ACR access-token auth.
const acrLoginUsername = "00000000-0000-0000-0000-000000000000"

type azureHelper struct{}

func (h *azureHelper) Matches(host string) bool {
	return strings.HasSuffix(host, ".azurecr.io")
}

func (h *azureHelper) BuildLoginContainer(host, repoName, tokenFilePath, credentialsMountPath string,
	cfg AuthConfig,
	imagePullPolicy string) (v1.Container, error) {

	registryName := strings.Split(host, ".")[0]

	// Exchange the federated token (from workload identity) for an az-cli session, if present.
	command := softFailScript(fmt.Sprintf(`if [ -n "$AZURE_FEDERATED_TOKEN_FILE" ]; then
  az login --service-principal -u "$AZURE_CLIENT_ID" -t "$AZURE_TENANT_ID" --federated-token "$(cat "$AZURE_FEDERATED_TOKEN_FILE")" >/dev/null
fi
%s`, writeCredentialFileScript(tokenFilePath, host, acrLoginUsername,
		fmt.Sprintf("az acr login --name %s --expose-token --output tsv --query accessToken", registryName))),
		host, "ACR")

	name, err := common.SanitizeKubernetesName("registry-login-azure-", host, false)
	if err != nil {
		return v1.Container{}, err
	}

	container := v1.Container{
		Name:            name,
		Image:           cfg.AzureCLIImage,
		ImagePullPolicy: v1.PullPolicy(imagePullPolicy),
		Command:         []string{"/bin/sh"},
		Args:            []string{"-c", command},
		VolumeMounts:    []v1.VolumeMount{TokenDirVolumeMount()},
	}

	if credentialsMountPath != "" {
		container.Env = append(container.Env, v1.EnvVar{
			Name:  "AZURE_AUTH_LOCATION",
			Value: credentialsMountPath + "/" + providerCredentialsFileName,
		})
	}

	return container, nil
}
