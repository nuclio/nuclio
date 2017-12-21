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

	"github.com/nuclio/nuclio/pkg/platform"
	"github.com/nuclio/nuclio/pkg/platform/kube"
	"github.com/nuclio/nuclio/pkg/platform/local"

	"github.com/nuclio/nuclio-sdk"
)

// CreatePlatform creates a platform based on a requested type (platformType) and configuration it receives
// and probes
func CreatePlatform(parentLogger nuclio.Logger,
	platformType string,
	platformConfiguration interface{}) (platform.Platform, error) {

	switch platformType {
	case "local":
		return local.NewPlatform(parentLogger)

	case "kube":
		return kube.NewPlatform(parentLogger, kube.GetKubeconfigPath(platformConfiguration))

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
		return nil, fmt.Errorf("Can't create platform - unsupported: %s", platformType)
	}
}
