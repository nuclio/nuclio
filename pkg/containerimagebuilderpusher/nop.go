/*
Copyright 2017 The Nuclio Authors.
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
