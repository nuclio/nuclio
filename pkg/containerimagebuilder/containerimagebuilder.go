package containerimagebuilder

// ContainerImageBuilderPusher is a builder of container images
type ContainerImageBuilderPusher interface {

	// BuildAndPushContainerImage builds container image and pushes it into container registry
	BuildAndPushContainerImage(buildOptions *BuildOptions, namespace string) error
}
