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

type ContainerBuilderConfiguration struct {
	Kind                                 string
	BusyBoxImage                         string
	AWSCLIImage                          string
	RegistryProviderSecretName           string
	KanikoImage                          string
	KanikoImagePullPolicy                string
	JobPrefix                            string
	JobDeletionTimeout                   time.Duration
	DefaultRegistryCredentialsSecretName string
	DefaultBaseRegistryURL               string
	DefaultOnbuildRegistryURL            string
	RegistryKind                         string
	DefaultServiceAccount                string
	CacheRepo                            string
	InsecurePushRegistry                 bool
	InsecurePullRegistry                 bool
	PushImagesRetries                    int
	ImageFSExtractionRetries             int

	// PodLabels are labels to set on the metadata of the build job pod
	// template, for either backend. Used, for example, to opt the pod into
	// the Azure Workload Identity webhook (azure.workload.identity/use:
	// "true") so it can authenticate to ACR via federated tokens on
	// identity-based installs.
	PodLabels map[string]string

	BuildahImage           string
	BuildahImagePullPolicy string

	// BuildahRootlessMode selects how the buildah container gets the
	// capabilities it needs to build images without running as root:
	// "caps" (default, SETUID/SETGID capabilities) or "hostusers"
	// (kubelet-owned user namespace, requires a modern k8s/containerd).
	BuildahRootlessMode string

	// BuildahStorageDriver is buildah's `--storage-driver` value: "overlay"
	// (default) or "vfs" (fallback for kernels/filesystems without overlay
	// support; has a per-layer performance penalty).
	BuildahStorageDriver string

	// BuildahIsolation is buildah's `--isolation` value: "chroot" (default,
	// no SYS_ADMIN needed) or "oci" (real namespace isolation per RUN step,
	// needs SYS_ADMIN or a user namespace).
	BuildahIsolation string

	// AzureCLIImage and GCloudCLIImage are the vendor CLI images used by the
	// registryhelpers azure/gcp providers to mint registry tokens.
	AzureCLIImage  string
	GCloudCLIImage string
}

func NewContainerBuilderConfiguration() (*ContainerBuilderConfiguration, error) {
	var containerBuilderConfiguration ContainerBuilderConfiguration
	var err error

	// if some of the parameters are undefined, try environment variables
	if containerBuilderConfiguration.Kind == "" {
		containerBuilderConfiguration.Kind = common.GetEnvOrDefaultString("NUCLIO_CONTAINER_BUILDER_KIND",
			"docker")
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
	if containerBuilderConfiguration.KanikoImage == "" {
		containerBuilderConfiguration.KanikoImage = common.GetEnvOrDefaultString("NUCLIO_KANIKO_CONTAINER_IMAGE",
			"gcr.io/kaniko-project/executor:v1.23.2")
	}
	if containerBuilderConfiguration.KanikoImagePullPolicy == "" {
		containerBuilderConfiguration.KanikoImagePullPolicy = common.GetEnvOrDefaultString(
			"NUCLIO_KANIKO_CONTAINER_IMAGE_PULL_POLICY", "IfNotPresent")
	}
	if containerBuilderConfiguration.JobPrefix == "" {
		containerBuilderConfiguration.JobPrefix = common.GetEnvOrDefaultString("NUCLIO_DASHBOARD_JOB_NAME_PREFIX",
			"buildjob")
	}

	containerBuilderConfiguration.InsecurePushRegistry =
		common.GetEnvOrDefaultBoolWithLegacyKey("NUCLIO_BUILD_INSECURE_PUSH_REGISTRY", "NUCLIO_KANIKO_INSECURE_PUSH_REGISTRY", false)
	containerBuilderConfiguration.InsecurePullRegistry =
		common.GetEnvOrDefaultBoolWithLegacyKey("NUCLIO_BUILD_INSECURE_PULL_REGISTRY", "NUCLIO_KANIKO_INSECURE_PULL_REGISTRY", false)

	containerBuilderConfiguration.DefaultRegistryCredentialsSecretName =
		common.GetEnvOrDefaultString("NUCLIO_REGISTRY_CREDENTIALS_SECRET_NAME", "")

	containerBuilderConfiguration.RegistryKind =
		common.GetEnvOrDefaultString("NUCLIO_REGISTRY_KIND", "")

	if containerBuilderConfiguration.DefaultBaseRegistryURL == "" {
		containerBuilderConfiguration.DefaultBaseRegistryURL =
			common.GetEnvOrDefaultString("NUCLIO_DASHBOARD_DEFAULT_BASE_REGISTRY_URL", "")
	}

	if containerBuilderConfiguration.DefaultOnbuildRegistryURL == "" {
		containerBuilderConfiguration.DefaultOnbuildRegistryURL =
			common.GetEnvOrDefaultString("NUCLIO_DASHBOARD_DEFAULT_ONBUILD_REGISTRY_URL", "quay.io")
	}

	containerBuilderConfiguration.CacheRepo =
		common.GetEnvOrDefaultString("NUCLIO_DASHBOARD_KANIKO_CACHE_REPO", "")

	containerBuilderConfiguration.PushImagesRetries, err =
		strconv.Atoi(common.GetEnvOrDefaultString("NUCLIO_KANIKO_PUSH_IMAGES_RETRIES", "3"))
	if err != nil {
		return nil, errors.Wrap(err, "Failed to resolve number of push images retries")
	}

	containerBuilderConfiguration.ImageFSExtractionRetries, err =
		strconv.Atoi(common.GetEnvOrDefaultString("NUCLIO_KANIKO_IMAGE_FS_EXTRACTION_RETRIES", "3"))
	if err != nil {
		return nil, errors.Wrap(err, "Failed to resolve number of push images retries")
	}

	jobDeletionTimeout := common.GetEnvOrDefaultStringWithLegacyKey(
		"NUCLIO_BUILD_JOB_DELETION_TIMEOUT", "NUCLIO_KANIKO_JOB_DELETION_TIMEOUT", "30m")
	containerBuilderConfiguration.JobDeletionTimeout, err = time.ParseDuration(jobDeletionTimeout)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to parse job deletion timeout duration")
	}

	containerBuilderConfiguration.DefaultServiceAccount = common.GetEnvOrDefaultStringWithLegacyKey(
		"NUCLIO_BUILD_DEFAULT_SERVICE_ACCOUNT", "NUCLIO_KANIKO_DEFAULT_SERVICE_ACCOUNT", "")

	if rawPodLabels := common.GetEnvOrDefaultStringWithLegacyKey(
		"NUCLIO_BUILD_POD_LABELS", "NUCLIO_KANIKO_POD_LABELS", ""); rawPodLabels != "" {
		if err := json.Unmarshal([]byte(rawPodLabels), &containerBuilderConfiguration.PodLabels); err != nil {
			return nil, errors.Wrap(err, "Failed to parse pod labels as JSON object")
		}
	}

	if containerBuilderConfiguration.BuildahImage == "" {
		containerBuilderConfiguration.BuildahImage = common.GetEnvOrDefaultString("NUCLIO_BUILDAH_CONTAINER_IMAGE",
			"quay.io/buildah/stable:v1.43.1")
	}
	if containerBuilderConfiguration.BuildahImagePullPolicy == "" {
		containerBuilderConfiguration.BuildahImagePullPolicy = common.GetEnvOrDefaultString(
			"NUCLIO_BUILDAH_CONTAINER_IMAGE_PULL_POLICY", "IfNotPresent")
	}

	if containerBuilderConfiguration.BuildahRootlessMode == "" {
		containerBuilderConfiguration.BuildahRootlessMode = common.GetEnvOrDefaultString("NUCLIO_BUILDAH_ROOTLESS_MODE",
			"caps")
	}
	if containerBuilderConfiguration.BuildahRootlessMode != "caps" && containerBuilderConfiguration.BuildahRootlessMode != "hostusers" {
		return nil, errors.Errorf("Invalid buildah rootless mode: %s (must be \"caps\" or \"hostusers\")",
			containerBuilderConfiguration.BuildahRootlessMode)
	}

	if containerBuilderConfiguration.BuildahStorageDriver == "" {
		containerBuilderConfiguration.BuildahStorageDriver = common.GetEnvOrDefaultString("NUCLIO_BUILDAH_STORAGE_DRIVER",
			"overlay")
	}
	if containerBuilderConfiguration.BuildahStorageDriver != "overlay" && containerBuilderConfiguration.BuildahStorageDriver != "vfs" {
		return nil, errors.Errorf("Invalid buildah storage driver: %s (must be \"overlay\" or \"vfs\")",
			containerBuilderConfiguration.BuildahStorageDriver)
	}

	if containerBuilderConfiguration.BuildahIsolation == "" {
		containerBuilderConfiguration.BuildahIsolation = common.GetEnvOrDefaultString("NUCLIO_BUILDAH_ISOLATION",
			"chroot")
	}
	if containerBuilderConfiguration.BuildahIsolation != "chroot" && containerBuilderConfiguration.BuildahIsolation != "oci" {
		return nil, errors.Errorf("Invalid buildah isolation: %s (must be \"chroot\" or \"oci\")",
			containerBuilderConfiguration.BuildahIsolation)
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
