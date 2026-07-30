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
	"strings"

	"github.com/nuclio/nuclio/pkg/common"

	v1 "k8s.io/api/core/v1"
)

// gcloudLoginUsername is the fixed username for GAR OAuth2 access tokens.
const gcloudLoginUsername = "oauth2accesstoken"

type gcpHelper struct{}

func (h *gcpHelper) Matches(host string) bool {
	return strings.HasSuffix(host, "-docker.pkg.dev")
}

func (h *gcpHelper) BuildLoginContainer(host, repoName, tokenFilePath, credentialsMountPath string,
	cfg AuthConfig,
	imagePullPolicy string) (v1.Container, error) {

	// GKE workload identity supplies ambient credentials; a mounted secret overrides via env below.
	command := softFailScript(
		writeCredentialFileScript(tokenFilePath, host, gcloudLoginUsername, "gcloud auth print-access-token"),
		host, "GAR")

	name, err := common.SanitizeKubernetesName("registry-login-gcp-", host, false)
	if err != nil {
		return v1.Container{}, err
	}

	container := v1.Container{
		Name:            name,
		Image:           cfg.GCloudCLIImage,
		ImagePullPolicy: v1.PullPolicy(imagePullPolicy),
		Command:         []string{"/bin/sh"},
		Args:            []string{"-c", command},
		VolumeMounts:    []v1.VolumeMount{TokenDirVolumeMount()},
	}

	if credentialsMountPath != "" {
		container.Env = append(container.Env, v1.EnvVar{
			Name:  "GOOGLE_APPLICATION_CREDENTIALS",
			Value: credentialsMountPath + "/" + providerCredentialsFileName,
		})
	}

	return container, nil
}
