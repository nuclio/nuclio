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
	"fmt"
	"strings"

	"github.com/nuclio/nuclio/pkg/platform"
	"github.com/nuclio/nuclio/pkg/platform/kube"
	"github.com/nuclio/nuclio/pkg/platform/local"

	"github.com/nuclio/logger"
)

// CreatePlatform creates a platform based on a requested type (platformType) and configuration it receives
// and probes
func CreatePlatform(parentLogger logger.Logger,
	platformType string,
	platformConfiguration interface{}) (platform.Platform, string, error) {

	switch platformType {
	case "local":
		platformInstance, err := local.NewPlatform(parentLogger)
		return platformInstance, platformType, err

	case "kube":
		platformInstance, err := kube.NewPlatform(parentLogger, kube.GetKubeconfigPath(platformConfiguration))
		return platformInstance, platformType, err

	case "auto":

		// try to get kubeconfig path
		kubeconfigPath := kube.GetKubeconfigPath(platformConfiguration)

		if kubeconfigPath != "" || kube.IsInCluster() {

			// call again, but force kube
			return CreatePlatform(parentLogger, "kube", platformConfiguration)
		}

		// call again, force local
		return CreatePlatform(parentLogger, "local", platformConfiguration)

	default:
		return nil, platformType, fmt.Errorf("Can't create platform - unsupported: %s", platformType)
	}
}

func InferExternalIPAddresses(logger logger.Logger,
	platformType string,
	externalIPAddresses string) ([]string, error) {

	if externalIPAddresses != "" {
		logger.DebugWith("User set external ip addresses", "externalIPAddresses", externalIPAddresses)
		return strings.Split(externalIPAddresses, ","), nil
	}

	switch platformType {
	case "local":

		// docker daemon ip
		defaultLocalIp := "172.17.0.1"
		logger.DebugWith("Platform type is local, setting docker daemon ip address as external ip address", "externalIPAddresses", defaultLocalIp)
		return []string{defaultLocalIp}, nil
	case "kube":
		logger.Debug(`Platform type is kube and no external ip addresses are passed, ` +
			`on function invocation the address will be the node's external/internal address`)
		return []string{}, nil
	}

	return []string{}, fmt.Errorf("Can't infer external ip address - unsupported platform type: %s", platformType)
}
