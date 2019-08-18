package containerimagebuilder

// ImageBuilderPusher is a builder of container images
type ImageBuilderPusher interface {

	// BuildAndPushContainerImage builds container image and pushes it into container registry
	BuildAndPushContainerImage(buildOptions *BuildOptions, namespace string) error
}
