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
)

// BuilderPusher is a builder of container images
type BuilderPusher interface {

	// GetKind returns the kind (docker/kaniko)
	GetKind() string

	// BuildAndPushContainerImage builds container image and pushes it into container registry
	BuildAndPushContainerImage(ctx context.Context, buildOptions *BuildOptions, namespace string) error

	// Get Onbuild stage for multistage builds
	GetOnbuildStages(onbuildArtifacts []runtime.Artifact) ([]string, error)

	// Change Onbuild artifact paths depending on the type of the builder used
	TransformOnbuildArtifactPaths(onbuildArtifacts []runtime.Artifact) (map[string]string, error)

	// GetBaseImageRegistry returns base image registry
	GetBaseImageRegistry(registry string) string

	// GetBaseImageRegistry returns onbuild base registry
	GetOnbuildImageRegistry(registry string) string

	// GetDefaultRegistryCredentialsSecretName returns secret with credentials to push/pull from docker registry
	GetDefaultRegistryCredentialsSecretName() string
}
