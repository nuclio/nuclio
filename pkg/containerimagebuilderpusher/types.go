package containerimagebuilderpusher

import "github.com/nuclio/nuclio/pkg/processor/build/runtime"

// BuildOptions are options for building a container image
type BuildOptions struct {
	Image               string
	ContextDir          string
	TempDir             string
	DockerfileInfo      *runtime.ProcessorDockerfileInfo
	NoCache             bool
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
	DefaultRegistryCredentialsSecretName string
	DefaultBaseRegistryURL               string
	DefaultOnbuildRegistryURL            string
	CacheRepo                            string
	InsecurePushRegistry                 bool
	InsecurePullRegistry                 bool
}
