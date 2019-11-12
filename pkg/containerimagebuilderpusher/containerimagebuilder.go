package containerimagebuilderpusher

import "github.com/nuclio/nuclio/pkg/processor/build/runtime"

// BuilderPusher is a builder of container images
type BuilderPusher interface {

	// BuildAndPushContainerImage builds container image and pushes it into container registry
	BuildAndPushContainerImage(buildOptions *BuildOptions, namespace string) error

	// Get Onbuild stage for multistage builds
	GetOnbuildStages(onbuildArtifacts []runtime.Artifact) ([]string, error)

	// Change Onbuild artifact paths depending on the type of the builder used
	TransformOnbuildArtifactPaths(onbuildArtifacts []runtime.Artifact) (map[string]string, error)
}
