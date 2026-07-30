/*
Copyright 2023 The Nuclio Authors.

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

package containerimagebuilderpusher

import (
	"encoding/json"
	"strconv"
	"time"

	"github.com/nuclio/nuclio/pkg/common"
	"github.com/nuclio/nuclio/pkg/containerimagebuilderpusher/registryhelpers"
	"github.com/nuclio/nuclio/pkg/processor/build/runtime"

	"github.com/nuclio/errors"
	"github.com/nuclio/logger"
	"k8s.io/api/core/v1"
)

// BuildOptions are options for building a container image
type BuildOptions struct {
	Image                                    string
	ContextDir                               string
	TempDir                                  string
	DockerfileInfo                           *runtime.ProcessorDockerfileInfo
	NoCache                                  bool
	Pull                                     bool
	NoBaseImagePull                          bool
	BuildFlags                               map[string]bool
	BuildArgs                                map[string]string
	RegistryURL                              string
	BaseImageRegistry                        string
	OnbuildImageRegistry                     string
	RepoName                                 string
	SecretName                               string
	OutputImageFile                          string
	BuildTimeoutSeconds                      int64
	Affinity                                 *v1.Affinity
	NodeSelector                             map[string]string
	NodeName                                 string
	PriorityClassName                        string
	Tolerations                              []v1.Toleration
	ReadinessTimeoutSeconds                  int
	FunctionServiceAccount                   string
	BuilderServiceAccount                    string
	SecurityContext                          *v1.PodSecurityContext
	Resources                                v1.ResourceRequirements
	ProjectName                              string
	ProjectSecretTemplate                    string
	ProjectSecretAllowedServiceAccountsKey   string
	ProjectSecretForbiddenServiceAccountsKey string
	ProjectSecretDefaultServiceAccountKey    string
	DefaultPlatformServiceAccount            string
	DefaultForbiddenServiceAccounts          []string

	BuildLogger logger.Logger
}

type KanikoConfig struct {
	Image           string
	ImagePullPolicy string

	ImageFSExtractionRetries int
}

type BuildahConfig struct {
	Image           string
	ImagePullPolicy string

	RootlessMode  string
	StorageDriver string
	Isolation     string

	AppArmorProfile string
}

type ContainerBuilderConfiguration struct {
	Kind                                  string
	JobPrefix                             string
	JobDeletionTimeout                    time.Duration
	DefaultRegistryCredentialsSecretName  string
	DefaultRegistryCredentialsSecretNames []string

	DefaultBaseRegistryURL    string
	DefaultOnbuildRegistryURL string
	RegistryKind              string
	DefaultServiceAccount     string
	CacheRepo                 string
	InsecurePushRegistry      bool
	InsecurePullRegistry      bool
	PushImagesRetries         int

	registryhelpers.AuthConfig

	BusyBoxImage string

	// PodLabels are extra labels set on the build job pod template, for either backend.
	PodLabels map[string]string

	// Builder specific configurations
	Kaniko  KanikoConfig
	Buildah BuildahConfig
}

// NewContainerBuilderConfiguration fills any field left unset on existing (e.g. from the platform
// config) from environment variables/defaults. Pass nil for env-only behavior.
func NewContainerBuilderConfiguration(existing *ContainerBuilderConfiguration) (*ContainerBuilderConfiguration, error) {
	var containerBuilderConfiguration ContainerBuilderConfiguration
	if existing != nil {
		containerBuilderConfiguration = *existing
	}
	var err error

	// if some of the parameters are undefined, try environment variables
	if containerBuilderConfiguration.Kind == "" {
		containerBuilderConfiguration.Kind = common.GetEnvOrDefaultString("NUCLIO_CONTAINER_BUILDER_KIND",
			DockerKind)
	}
	if containerBuilderConfiguration.BusyBoxImage == "" {
		containerBuilderConfiguration.BusyBoxImage = common.GetEnvOrDefaultString("NUCLIO_BUSYBOX_CONTAINER_IMAGE",
			"busybox:stable")
	}
	if containerBuilderConfiguration.AWSCLIImage == "" {
		containerBuilderConfiguration.AWSCLIImage = common.GetEnvOrDefaultString("NUCLIO_AWS_CLI_CONTAINER_IMAGE",
			"amazon/aws-cli:2.17.16")
	}
	if containerBuilderConfiguration.RegistryProviderSecretName == "" {
		containerBuilderConfiguration.RegistryProviderSecretName = common.GetEnvOrDefaultStringWithLegacyKey(
			"NUCLIO_BUILD_REGISTRY_PROVIDER_AUTH_SECRET_NAME", "NUCLIO_KANIKO_REGISTRY_PROVIDER_AUTH_SECRET_NAME", "")
	}
	if containerBuilderConfiguration.Kaniko.Image == "" {
		containerBuilderConfiguration.Kaniko.Image = common.GetEnvOrDefaultString("NUCLIO_KANIKO_CONTAINER_IMAGE",
			"gcr.io/kaniko-project/executor:v1.23.2")
	}
	if containerBuilderConfiguration.Kaniko.ImagePullPolicy == "" {
		containerBuilderConfiguration.Kaniko.ImagePullPolicy = common.GetEnvOrDefaultString(
			"NUCLIO_KANIKO_CONTAINER_IMAGE_PULL_POLICY", "IfNotPresent")
	}
	if containerBuilderConfiguration.JobPrefix == "" {
		containerBuilderConfiguration.JobPrefix = common.GetEnvOrDefaultString("NUCLIO_DASHBOARD_JOB_NAME_PREFIX",
			"buildjob")
	}

	if !containerBuilderConfiguration.InsecurePushRegistry {
		containerBuilderConfiguration.InsecurePushRegistry =
			common.GetEnvOrDefaultBoolWithLegacyKey("NUCLIO_BUILD_INSECURE_PUSH_REGISTRY", "NUCLIO_KANIKO_INSECURE_PUSH_REGISTRY", false)
	}
	if !containerBuilderConfiguration.InsecurePullRegistry {
		containerBuilderConfiguration.InsecurePullRegistry =
			common.GetEnvOrDefaultBoolWithLegacyKey("NUCLIO_BUILD_INSECURE_PULL_REGISTRY", "NUCLIO_KANIKO_INSECURE_PULL_REGISTRY", false)
	}

	if containerBuilderConfiguration.DefaultRegistryCredentialsSecretName == "" {
		containerBuilderConfiguration.DefaultRegistryCredentialsSecretName =
			common.GetEnvOrDefaultString("NUCLIO_REGISTRY_CREDENTIALS_SECRET_NAME", "")
	}

	// merge singular + list + env list into one ordered, deduped list
	secretNames := append([]string{}, containerBuilderConfiguration.DefaultRegistryCredentialsSecretNames...)
	secretNames = append(secretNames, common.GetEnvOrDefaultStringSlice("NUCLIO_REGISTRY_CREDENTIALS_SECRET_NAMES", nil)...)
	if containerBuilderConfiguration.DefaultRegistryCredentialsSecretName != "" {
		secretNames = append([]string{containerBuilderConfiguration.DefaultRegistryCredentialsSecretName}, secretNames...)
	}
	containerBuilderConfiguration.DefaultRegistryCredentialsSecretNames = common.RemoveDuplicatesFromSliceString(secretNames)

	if containerBuilderConfiguration.PythonImage == "" {
		containerBuilderConfiguration.PythonImage =
			common.GetEnvOrDefaultString("NUCLIO_PYTHON_BASE_IMAGE_NAME", "gcr.io/iguazio/python:3.11")
	}

	if containerBuilderConfiguration.PythonImagePullPolicy == "" {
		containerBuilderConfiguration.PythonImagePullPolicy =
			common.GetEnvOrDefaultString("NUCLIO_PYTHON_BASE_IMAGE_PULL_POLICY", "IfNotPresent")
	}

	if containerBuilderConfiguration.RegistryKind == "" {
		containerBuilderConfiguration.RegistryKind =
			common.GetEnvOrDefaultString("NUCLIO_REGISTRY_KIND", "")
	}

	if containerBuilderConfiguration.DefaultBaseRegistryURL == "" {
		containerBuilderConfiguration.DefaultBaseRegistryURL =
			common.GetEnvOrDefaultString("NUCLIO_DASHBOARD_DEFAULT_BASE_REGISTRY_URL", "")
	}

	if containerBuilderConfiguration.DefaultOnbuildRegistryURL == "" {
		containerBuilderConfiguration.DefaultOnbuildRegistryURL =
			common.GetEnvOrDefaultString("NUCLIO_DASHBOARD_DEFAULT_ONBUILD_REGISTRY_URL", "quay.io")
	}

	if containerBuilderConfiguration.CacheRepo == "" {
		containerBuilderConfiguration.CacheRepo = common.GetEnvOrDefaultStringWithLegacyKey(
			"NUCLIO_BUILD_CACHE_REPO", "NUCLIO_DASHBOARD_KANIKO_CACHE_REPO", "")
	}

	if containerBuilderConfiguration.PushImagesRetries == 0 {
		containerBuilderConfiguration.PushImagesRetries, err =
			strconv.Atoi(common.GetEnvOrDefaultStringWithLegacyKey(
				"NUCLIO_BUILD_PUSH_IMAGES_RETRIES", "NUCLIO_KANIKO_PUSH_IMAGES_RETRIES", "3"))
		if err != nil {
			return nil, errors.Wrap(err, "Failed to resolve number of push images retries")
		}
	}

	if containerBuilderConfiguration.Kaniko.ImageFSExtractionRetries == 0 {
		containerBuilderConfiguration.Kaniko.ImageFSExtractionRetries, err =
			strconv.Atoi(common.GetEnvOrDefaultString("NUCLIO_KANIKO_IMAGE_FS_EXTRACTION_RETRIES", "3"))
		if err != nil {
			return nil, errors.Wrap(err, "Failed to resolve number of image filesystem extraction retries")
		}
	}

	if containerBuilderConfiguration.JobDeletionTimeout == 0 {
		jobDeletionTimeout := common.GetEnvOrDefaultStringWithLegacyKey(
			"NUCLIO_BUILD_JOB_DELETION_TIMEOUT", "NUCLIO_KANIKO_JOB_DELETION_TIMEOUT", "30m")
		containerBuilderConfiguration.JobDeletionTimeout, err = time.ParseDuration(jobDeletionTimeout)
		if err != nil {
			return nil, errors.Wrap(err, "Failed to parse job deletion timeout duration")
		}
	}

	if containerBuilderConfiguration.DefaultServiceAccount == "" {
		containerBuilderConfiguration.DefaultServiceAccount = common.GetEnvOrDefaultStringWithLegacyKey(
			"NUCLIO_BUILD_DEFAULT_SERVICE_ACCOUNT", "NUCLIO_KANIKO_DEFAULT_SERVICE_ACCOUNT", "")
	}

	if len(containerBuilderConfiguration.PodLabels) == 0 {
		if rawPodLabels := common.GetEnvOrDefaultStringWithLegacyKey(
			"NUCLIO_BUILD_POD_LABELS", "NUCLIO_KANIKO_POD_LABELS", ""); rawPodLabels != "" {
			if err := json.Unmarshal([]byte(rawPodLabels), &containerBuilderConfiguration.PodLabels); err != nil {
				return nil, errors.Wrap(err, "Failed to parse pod labels as JSON object")
			}
		}
	}

	if containerBuilderConfiguration.Buildah.Image == "" {
		containerBuilderConfiguration.Buildah.Image = common.GetEnvOrDefaultString("NUCLIO_BUILDAH_CONTAINER_IMAGE",
			"quay.io/buildah/stable:v1.43.1")
	}
	if containerBuilderConfiguration.Buildah.ImagePullPolicy == "" {
		containerBuilderConfiguration.Buildah.ImagePullPolicy = common.GetEnvOrDefaultString(
			"NUCLIO_BUILDAH_CONTAINER_IMAGE_PULL_POLICY", "IfNotPresent")
	}

	if containerBuilderConfiguration.Buildah.RootlessMode == "" {
		containerBuilderConfiguration.Buildah.RootlessMode = common.GetEnvOrDefaultString("NUCLIO_BUILDAH_ROOTLESS_MODE",
			"caps")
	}
	if containerBuilderConfiguration.Buildah.RootlessMode != "caps" && containerBuilderConfiguration.Buildah.RootlessMode != "hostusers" {
		return nil, errors.Errorf("Invalid buildah rootless mode: %s (must be \"caps\" or \"hostusers\")",
			containerBuilderConfiguration.Buildah.RootlessMode)
	}

	if containerBuilderConfiguration.Buildah.StorageDriver == "" {
		containerBuilderConfiguration.Buildah.StorageDriver = common.GetEnvOrDefaultString("NUCLIO_BUILDAH_STORAGE_DRIVER",
			"overlay")
	}
	if containerBuilderConfiguration.Buildah.StorageDriver != "overlay" && containerBuilderConfiguration.Buildah.StorageDriver != "vfs" {
		return nil, errors.Errorf("Invalid buildah storage driver: %s (must be \"overlay\" or \"vfs\")",
			containerBuilderConfiguration.Buildah.StorageDriver)
	}

	if containerBuilderConfiguration.Buildah.Isolation == "" {
		containerBuilderConfiguration.Buildah.Isolation = common.GetEnvOrDefaultString("NUCLIO_BUILDAH_ISOLATION",
			"chroot")
	}
	if containerBuilderConfiguration.Buildah.Isolation != "chroot" && containerBuilderConfiguration.Buildah.Isolation != "oci" {
		return nil, errors.Errorf("Invalid buildah isolation: %s (must be \"chroot\" or \"oci\")",
			containerBuilderConfiguration.Buildah.Isolation)
	}

	if containerBuilderConfiguration.Buildah.AppArmorProfile == "" {
		containerBuilderConfiguration.Buildah.AppArmorProfile = common.GetEnvOrDefaultString(
			"NUCLIO_BUILDAH_APPARMOR_PROFILE", "unconfined")
	}

	if containerBuilderConfiguration.AzureCLIImage == "" {
		containerBuilderConfiguration.AzureCLIImage = common.GetEnvOrDefaultString("NUCLIO_AZURE_CLI_CONTAINER_IMAGE",
			"mcr.microsoft.com/azure-cli:2.88.0")
	}
	if containerBuilderConfiguration.GCloudCLIImage == "" {
		containerBuilderConfiguration.GCloudCLIImage = common.GetEnvOrDefaultString("NUCLIO_GCLOUD_CLI_CONTAINER_IMAGE",
			"google/cloud-sdk:576.0.0-slim")
	}

	return &containerBuilderConfiguration, nil
}
