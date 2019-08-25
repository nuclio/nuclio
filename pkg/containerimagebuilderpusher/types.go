package containerimagebuilderpusher

// BuildOptions are options for building a container image
type BuildOptions struct {
	Image           string
	ContextDir      string
	TempDir         string
	DockerfilePath  string
	NoCache         bool
	BuildArgs       map[string]string
	RegistryURL     string
	OutputImageFile string
}

type ContainerBuilderConfiguration struct {
	Kind             string
	BusyBoxImage     string
	KanikoImage      string
	JobPrefix        string
	InsecureRegistry bool
}
