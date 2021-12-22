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

package factory

import (
	"context"

	"github.com/nuclio/nuclio/pkg/common"
	"github.com/nuclio/nuclio/pkg/containerimagebuilderpusher"
	"github.com/nuclio/nuclio/pkg/platform"
	"github.com/nuclio/nuclio/pkg/platform/kube"
	"github.com/nuclio/nuclio/pkg/platform/local"
	"github.com/nuclio/nuclio/pkg/platformconfig"

	"github.com/nuclio/errors"
	"github.com/nuclio/logger"
)

// CreatePlatform creates a platform based on a requested type (platformType) and configuration it receives
// and probes
func CreatePlatform(ctx context.Context,
	parentLogger logger.Logger,
	platformType string,
	platformConfiguration *platformconfig.Config,
	defaultNamespace string) (platform.Platform, error) {

	var newPlatform platform.Platform
	var err error

	platformConfiguration.ContainerBuilderConfiguration, err =
		containerimagebuilderpusher.NewContainerBuilderConfiguration()
	if err != nil {
		return nil, errors.Wrapf(err, "Failed to create %s platform", platformType)
	}

	switch platformType {
	case "local":
		newPlatform, err = local.NewPlatform(ctx, parentLogger, platformConfiguration, defaultNamespace)

	case "kube":
		newPlatform, err = kube.NewPlatform(ctx, parentLogger, platformConfiguration, defaultNamespace)

	case "auto":

		// kubeconfig path is set, or running in kubernetes cluster
		if common.GetKubeconfigPath(platformConfiguration.Kube.KubeConfigPath) != "" ||
			common.IsInKubernetesCluster() {

			// call again, but force kube
			newPlatform, err = CreatePlatform(ctx, parentLogger, "kube", platformConfiguration, defaultNamespace)
		} else {

			// call again, force local
			newPlatform, err = CreatePlatform(ctx, parentLogger, "local", platformConfiguration, defaultNamespace)
		}

	default:
		return nil, errors.Errorf("Can't create platform - unsupported: %s", platformType)
	}

	if err != nil {
		return nil, errors.Wrapf(err, "Failed to create %s platform", platformType)
	}

	// under this section, add actions to be performed only after platform type had been resolved
	// (so it won't be performed more than once)
	if platformType != "auto" {
		parentLogger.DebugWithCtx(ctx, "Initializing platform", "platformType", platformType)
		if err = newPlatform.Initialize(ctx); err != nil {
			return nil, errors.Wrap(err, "Failed to initialize platform")
		}
	}

	return newPlatform, nil
}
