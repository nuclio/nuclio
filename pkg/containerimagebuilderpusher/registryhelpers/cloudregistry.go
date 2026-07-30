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

// Package registryhelpers authenticates to cloud registry vendors and merges docker-config
// secrets and cloud credentials into one authfile for the buildah and kaniko backends.
package registryhelpers

import (
	"fmt"

	"github.com/nuclio/errors"
	v1 "k8s.io/api/core/v1"
)

const (
	// AuthVolumeName is the authfile volume's name.
	AuthVolumeName = "authdir"

	authTokensVolumeName = "authtokens"
	authTokensMountPath  = "/tmp/registry-auth-tokens"

	providerCredentialsMountPath = "/tmp/registry-provider-credentials"

	// providerCredentialsFileName is the key every provider secret carries, after AWS's
	// ~/.aws/credentials convention; used the same way for Azure and GCP.
	providerCredentialsFileName = "credentials"
)

// AuthConfig carries the images and provider secret the auth/merge init containers need.
type AuthConfig struct {
	AWSCLIImage                string
	AzureCLIImage              string
	GCloudCLIImage             string
	RegistryProviderSecretName string
	PythonImage                string
	PythonImagePullPolicy      string
}

// CloudRegistryHelper authenticates to one cloud registry vendor.
type CloudRegistryHelper interface {
	// Matches reports whether host belongs to this vendor.
	Matches(host string) bool

	// BuildLoginContainer returns the init container writing host's token to tokenFilePath. repoName
	// is set only for the push destination, for vendors that provision repos (ECR);
	// credentialsMountPath is where the static provider secret is mounted, or "" if none.
	BuildLoginContainer(host, repoName, tokenFilePath, credentialsMountPath string,
		cfg AuthConfig,
		imagePullPolicy string) (v1.Container, error)
}

// cloudRegistryHelpers is the static, ordered set of supported vendors.
var cloudRegistryHelpers = []CloudRegistryHelper{&awsHelper{}, &azureHelper{}, &gcpHelper{}}

// helperFor returns the CloudRegistryHelper matching host, or nil.
func helperFor(host string) CloudRegistryHelper {
	for _, helper := range cloudRegistryHelpers {
		if helper.Matches(host) {
			return helper
		}
	}
	return nil
}

// NeedsCloudLogin reports whether any of hosts is a supported cloud registry.
func NeedsCloudLogin(hosts []string) bool {
	for _, host := range hosts {
		if helperFor(host) != nil {
			return true
		}
	}
	return false
}

// TokenDirVolumeMount is the shared dir login containers write credential files into.
func TokenDirVolumeMount() v1.VolumeMount {
	return v1.VolumeMount{Name: authTokensVolumeName, MountPath: authTokensMountPath}
}

// BuildLoginContainers returns one login init container per host that resolves to a cloud vendor.
func BuildLoginContainers(hosts []string,
	pushDestination string,
	repoName string,
	cfg AuthConfig,
	imagePullPolicy string) ([]v1.Container, []v1.Volume, error) {

	pushHost := hostOf(pushDestination)
	tokenDir := TokenDirVolumeMount()

	var volumes []v1.Volume
	credentialsMountPath := ""
	if cfg.RegistryProviderSecretName != "" {
		credentialsMountPath = providerCredentialsMountPath
		volumes = append(volumes, v1.Volume{
			Name:         cfg.RegistryProviderSecretName,
			VolumeSource: v1.VolumeSource{Secret: &v1.SecretVolumeSource{SecretName: cfg.RegistryProviderSecretName}},
		})
	}

	var containers []v1.Container
	tokenIndex := 0

	for _, host := range hosts {
		helper := helperFor(host)
		if helper == nil {
			continue
		}

		hostRepoName := ""
		if host == pushHost {
			hostRepoName = repoName
		}
		tokenFilePath := fmt.Sprintf("%s/%d.token", tokenDir.MountPath, tokenIndex)

		container, err := helper.BuildLoginContainer(host, hostRepoName, tokenFilePath, credentialsMountPath, cfg, imagePullPolicy)
		if err != nil {
			return nil, nil, errors.Wrapf(err, "Failed to build registry login init container for host: %s", host)
		}
		if credentialsMountPath != "" {
			container.VolumeMounts = append(container.VolumeMounts,
				v1.VolumeMount{Name: cfg.RegistryProviderSecretName, MountPath: credentialsMountPath})
		}

		containers = append(containers, container)
		tokenIndex++
	}

	return containers, volumes, nil
}
