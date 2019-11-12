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
	OutputImageFile     string
	BuildTimeoutSeconds int64
}

type ContainerBuilderConfiguration struct {
	Kind             string
	BusyBoxImage     string
	KanikoImage      string
	JobPrefix        string
	InsecureRegistry bool
}
