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
	"regexp"
	"strings"

	"github.com/nuclio/nuclio/pkg/common"

	v1 "k8s.io/api/core/v1"
)

// ecrLoginUsername is the fixed username for ECR get-login-password tokens.
const ecrLoginUsername = "AWS"

// ecrHostPattern matches an ECR private registry hostname, e.g. <accountId>.dkr.ecr.<region>.amazonaws.com.
var ecrHostPattern = regexp.MustCompile(`^\d{12}\.dkr\.ecr(-fips)?\.[a-z0-9-]+\.amazonaws\.com(\.cn)?$`)

// AWSHelper authenticates to ECR and exposes ECR URL parsing that both the buildah
// login-container flow and kaniko's bundled credential helper need.
type AWSHelper struct{}

// Matches detects an ECR hostname, e.g. <registryId>.dkr.ecr.<region>.amazonaws.com.
func (h *AWSHelper) Matches(host string) bool {
	return ecrHostPattern.MatchString(host)
}

// ECRRegion extracts the AWS region from an ECR registry URL.
func (h *AWSHelper) ECRRegion(url string) string {
	return strings.Split(url, ".")[3]
}

// ECRRegistryID extracts the AWS account ID (registry ID) from an ECR registry URL.
func (h *AWSHelper) ECRRegistryID(url string) string {
	return strings.Split(url, ".")[0]
}

func (h *AWSHelper) BuildLoginContainer(host, repoName, tokenFilePath, credentialsMountPath, imagePullPolicy string,
	cfg AuthConfig) (v1.Container, error) {

	envVars := append(credentialFileEnv(host, ecrLoginUsername, tokenFilePath),
		v1.EnvVar{Name: envVarECRRegion, Value: h.ECRRegion(host)},
		v1.EnvVar{Name: envVarECRRegistryID, Value: h.ECRRegistryID(host)})

	var commandParts []string
	// Pulling a base/onbuild image never needs repo creation, only the push destination does.
	if repoName != "" {
		envVars = append(envVars, v1.EnvVar{Name: envVarRegistryRepo, Value: repoName})
		commandParts = append(commandParts, fmt.Sprintf(`(set -e
aws ecr create-repository --repository-name "$%s" --region "$%s" --registry-id "$%s"
aws ecr create-repository --repository-name "$%s"/cache --region "$%s" --registry-id "$%s"
) || echo "WARNING: failed to ensure ECR repository $%s exists" >&2`,
			envVarRegistryRepo, envVarECRRegion, envVarECRRegistryID,
			envVarRegistryRepo, envVarECRRegion, envVarECRRegistryID,
			envVarRegistryRepo))
	}
	commandParts = append(commandParts, softFailScript(
		writeCredentialFileScript(
			fmt.Sprintf(`aws ecr get-login-password --region "$%s"`, envVarECRRegion)),
		"ECR"))

	name, err := common.SanitizeKubernetesName("registry-login-aws-", host, false)
	if err != nil {
		return v1.Container{}, err
	}

	container := v1.Container{
		Name:            name,
		Image:           cfg.AWSCLIImage,
		ImagePullPolicy: v1.PullPolicy(imagePullPolicy),
		Command:         []string{"/bin/sh"},
		Args:            []string{"-c", strings.Join(commandParts, "\n")},
		Env:             envVars,
		VolumeMounts:    []v1.VolumeMount{TokenDirVolumeMount()},
	}

	if credentialsMountPath != "" {
		container.Env = append(container.Env, v1.EnvVar{
			Name:  "AWS_SHARED_CREDENTIALS_FILE",
			Value: credentialsMountPath + "/" + providerCredentialsFileName,
		})
	} else {
		// ambient credentials (e.g. IRSA) picked up from the pod's service account
		container.Env = append(container.Env, v1.EnvVar{
			Name:  "AWS_SDK_LOAD_CONFIG",
			Value: "true",
		})
	}

	return container, nil
}
