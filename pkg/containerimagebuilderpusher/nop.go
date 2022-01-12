package containerimagebuilderpusher

import (
	"context"

	"github.com/nuclio/nuclio/pkg/processor/build/runtime"

	"github.com/nuclio/logger"
)

type Nop struct {
	logger logger.Logger
}

func NewNop(logger logger.Logger, builderConfiguration *ContainerBuilderConfiguration) (BuilderPusher, error) {
	nop := Nop{
		logger: logger,
	}
	return nop, nil
}

func (n Nop) GetKind() string {
	return "nop"
}

func (n Nop) BuildAndPushContainerImage(ctx context.Context, buildOptions *BuildOptions, namespace string) error {
	return nil
}

func (n Nop) GetOnbuildStages(onbuildArtifacts []runtime.Artifact) ([]string, error) {
	return nil, nil
}

func (n Nop) TransformOnbuildArtifactPaths(onbuildArtifacts []runtime.Artifact) (map[string]string, error) {
	return nil, nil
}

func (n Nop) GetBaseImageRegistry(registry string) string {
	return ""
}

func (n Nop) GetOnbuildImageRegistry(registry string) string {
	return ""
}

func (n Nop) GetDefaultRegistryCredentialsSecretName() string {
	return ""
}
