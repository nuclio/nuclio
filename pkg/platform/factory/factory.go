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
