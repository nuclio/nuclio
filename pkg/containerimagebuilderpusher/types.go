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
	"strconv"
	"time"

	"github.com/nuclio/nuclio/pkg/common"
	"github.com/nuclio/nuclio/pkg/processor/build/runtime"

	"github.com/nuclio/errors"
	"k8s.io/api/core/v1"
)

// BuildOptions are options for building a container image
type BuildOptions struct {
	Image                   string
	ContextDir              string
	TempDir                 string
	DockerfileInfo          *runtime.ProcessorDockerfileInfo
	NoCache                 bool
	Pull                    bool
	NoBaseImagePull         bool
	BuildFlags              map[string]bool
	BuildArgs               map[string]string
	RegistryURL             string
	RepoName                string
	SecretName              string
	OutputImageFile         string
	BuildTimeoutSeconds     int64
	Affinity                *v1.Affinity
	NodeSelector            map[string]string
	NodeName                string
	PriorityClassName       string
	Tolerations             []v1.Toleration
	ReadinessTimeoutSeconds int
	FunctionServiceAccount  string
	BuilderServiceAccount   string
	SecurityContext         *v1.PodSecurityContext
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
			"amazon/aws-cli:2.7.10")
	}
	if containerBuilderConfiguration.RegistryProviderSecretName == "" {
		containerBuilderConfiguration.RegistryProviderSecretName = common.GetEnvOrDefaultString("NUCLIO_KANIKO_REGISTRY_PROVIDER_AUTH_SECRET_NAME",
			"")
	}
	if containerBuilderConfiguration.KanikoImage == "" {
		containerBuilderConfiguration.KanikoImage = common.GetEnvOrDefaultString("NUCLIO_KANIKO_CONTAINER_IMAGE",
			"gcr.io/kaniko-project/executor:v1.9.0")
	}
	if containerBuilderConfiguration.KanikoImagePullPolicy == "" {
		containerBuilderConfiguration.KanikoImagePullPolicy = common.GetEnvOrDefaultString(
			"NUCLIO_KANIKO_CONTAINER_IMAGE_PULL_POLICY", "IfNotPresent")
	}
	if containerBuilderConfiguration.JobPrefix == "" {
		containerBuilderConfiguration.JobPrefix = common.GetEnvOrDefaultString("NUCLIO_DASHBOARD_JOB_NAME_PREFIX",
			"kanikojob")
	}

	containerBuilderConfiguration.InsecurePushRegistry =
		common.GetEnvOrDefaultBool("NUCLIO_KANIKO_INSECURE_PUSH_REGISTRY", false)
	containerBuilderConfiguration.InsecurePullRegistry =
		common.GetEnvOrDefaultBool("NUCLIO_KANIKO_INSECURE_PULL_REGISTRY", false)

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

	jobDeletionTimeout := common.GetEnvOrDefaultString("NUCLIO_KANIKO_JOB_DELETION_TIMEOUT", "30m")
	containerBuilderConfiguration.JobDeletionTimeout, err = time.ParseDuration(jobDeletionTimeout)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to parse job deletion timeout duration")
	}

	containerBuilderConfiguration.DefaultServiceAccount = common.GetEnvOrDefaultString("NUCLIO_KANIKO_DEFAULT_SERVICE_ACCOUNT",
		"")

	return &containerBuilderConfiguration, nil
}
