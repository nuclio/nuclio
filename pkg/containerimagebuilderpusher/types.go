package containerimagebuilderpusher

import (
	"strconv"
	"time"

	"github.com/nuclio/nuclio/pkg/common"
	"github.com/nuclio/nuclio/pkg/processor/build/runtime"

	"github.com/nuclio/errors"
)

// BuildOptions are options for building a container image
type BuildOptions struct {
	Image               string
	ContextDir          string
	TempDir             string
	DockerfileInfo      *runtime.ProcessorDockerfileInfo
	NoCache             bool
	Pull                bool
	NoBaseImagePull     bool
	BuildArgs           map[string]string
	RegistryURL         string
	SecretName          string
	OutputImageFile     string
	BuildTimeoutSeconds int64
}

type ContainerBuilderConfiguration struct {
	Kind                                 string
	BusyBoxImage                         string
	KanikoImage                          string
	KanikoImagePullPolicy                string
	JobPrefix                            string
	JobDeletionTimeout                   time.Duration
	DefaultRegistryCredentialsSecretName string
	DefaultBaseRegistryURL               string
	DefaultOnbuildRegistryURL            string
	CacheRepo                            string
	InsecurePushRegistry                 bool
	InsecurePullRegistry                 bool
	PushImagesRetries                    int
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
			"busybox:1.31")
	}
	if containerBuilderConfiguration.KanikoImage == "" {
		containerBuilderConfiguration.KanikoImage = common.GetEnvOrDefaultString("NUCLIO_KANIKO_CONTAINER_IMAGE",
			"gcr.io/kaniko-project/executor:v1.7.0")
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

	jobDeletionTimeout := common.GetEnvOrDefaultString("NUCLIO_KANIKO_JOB_DELETION_TIMEOUT", "30m")
	containerBuilderConfiguration.JobDeletionTimeout, err = time.ParseDuration(jobDeletionTimeout)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to parse job deletion timeout duration")
	}

	return &containerBuilderConfiguration, nil
}
