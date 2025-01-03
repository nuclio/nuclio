/*
Copyright 2023 The Nuclio Authors.

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
	version "github.com/v3io/version-go"
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

	platformType, err = GetPlatformByType(platformType, platformConfiguration)
	if err != nil {
		return nil, errors.Wrapf(err, "Failed to create %s platform", platformType)
	}
	switch platformType {
	case common.LocalPlatformName:
		newPlatform, err = local.NewPlatform(ctx, parentLogger, platformConfiguration, defaultNamespace)

	case common.KubePlatformName:
		newPlatform, err = kube.NewPlatform(ctx, parentLogger, platformConfiguration, defaultNamespace)

	default:

		// should not get here. see how GetPlatformByType ensures platformType can be only one of the above
		return nil, errors.Errorf("Can't create platform - unsupported: %s", platformType)
	}

	// check platform creation error
	if err != nil {
		return nil, errors.Wrapf(err, "Failed to create %s platform", platformType)
	}

	parentLogger.DebugWithCtx(ctx,
		"Initializing platform",
		"version", version.Get().String(),
		"platformName", newPlatform.GetName())
	if err = newPlatform.Initialize(ctx); err != nil {
		return nil, errors.Wrap(err, "Failed to initialize platform")
	}

	return newPlatform, nil
}

func GetPlatformByType(platformType string,
	platformConfiguration *platformconfig.Config) (string, error) {

	switch platformType {
	case common.LocalPlatformName:
		return common.LocalPlatformName, nil

	case common.KubePlatformName:
		return common.KubePlatformName, nil

	case common.AutoPlatformName:

		// kubeconfig path is set, or running in kubernetes cluster
		if common.GetKubeconfigPath(platformConfiguration.Kube.KubeConfigPath) != "" ||
			common.IsInKubernetesCluster() {
			return common.KubePlatformName, nil
		}
		return common.LocalPlatformName, nil

	default:
		return "", errors.Errorf("Can't create platform - unsupported: %s", platformType)
	}
}
