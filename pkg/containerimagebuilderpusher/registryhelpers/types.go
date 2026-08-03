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

// Env var names the login scripts read their values from, instead of having them interpolated
// into the script text.
const (
	envVarRegistryHost      = "REGISTRY_HOST"
	envVarRegistryUsername  = "REGISTRY_USERNAME"
	envVarRegistryRepo      = "REGISTRY_REPO"
	envVarRegistryTokenFile = "REGISTRY_TOKEN_FILE"
	envVarECRRegion         = "ECR_REGION"
	envVarECRRegistryID     = "ECR_REGISTRY_ID"
	envVarACRRegistryName   = "ACR_REGISTRY_NAME"
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
