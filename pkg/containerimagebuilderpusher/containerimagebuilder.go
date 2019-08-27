package containerimagebuilderpusher

// BuilderPusher is a builder of container images
type BuilderPusher interface {

	// BuildAndPushContainerImage builds container image and pushes it into container registry
	BuildAndPushContainerImage(buildOptions *BuildOptions, namespace string) error
}
