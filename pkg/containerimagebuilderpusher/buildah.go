/*
Copyright 2026 The Nuclio Authors.

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

	"github.com/nuclio/nuclio/pkg/platform/kube/clients/kube"

	"github.com/nuclio/errors"
	"github.com/nuclio/logger"
)

const buildahKind = "buildah"

type Buildah struct {
	*jobRunner
}

func NewBuildah(logger logger.Logger,
	kubeClientSet kube.Client,
	builderConfiguration *ContainerBuilderConfiguration) (*Buildah, error) {

	jr, err := newJobRunner(buildahKind, logger, kubeClientSet, builderConfiguration)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to create buildah job runner")
	}

	return &Buildah{jobRunner: jr}, nil
}

func (b *Buildah) BuildAndPushContainerImage(ctx context.Context,
	buildOptions *BuildOptions,
	namespace string) error {
	return errors.New("Buildah build/push is not implemented yet")
}
