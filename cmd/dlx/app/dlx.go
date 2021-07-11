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

package app

import (
	"github.com/nuclio/nuclio/pkg/common"
	"github.com/nuclio/nuclio/pkg/loggersink"
	nuclioioclient "github.com/nuclio/nuclio/pkg/platform/kube/client/clientset/versioned"
	"github.com/nuclio/nuclio/pkg/platform/kube/resourcescaler"
	"github.com/nuclio/nuclio/pkg/platformconfig"
	// load all sinks
	_ "github.com/nuclio/nuclio/pkg/sinks"

	"github.com/nuclio/errors"
	"github.com/v3io/scaler/pkg/dlx"
)

func Run(platformConfigurationPath string, namespace string, kubeconfigPath string) error {

	// create dlx
	dlxInstance, err := newDLX(platformConfigurationPath, namespace, kubeconfigPath)
	if err != nil {
		return errors.Wrap(err, "Failed to create dlx")
	}

	// start dlx and run forever
	if err = dlxInstance.Start(); err != nil {
		return errors.Wrap(err, "Failed to start dlx")
	}
	select {}
}

func newDLX(platformConfigurationPath string, namespace string, kubeconfigPath string) (*dlx.DLX, error) {

	// get platform configuration
	platformConfiguration, err := platformconfig.NewPlatformConfig(platformConfigurationPath)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to get platform configuration")
	}

	// create root logger
	rootLogger, err := loggersink.CreateSystemLogger("autoscaler", platformConfiguration)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to create logger")
	}

	restConfig, err := common.GetClientConfig(kubeconfigPath)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to get client configuration")
	}

	nuclioClientSet, err := nuclioioclient.NewForConfig(restConfig)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to create nuclio client set")
	}

	// create resource scaler
	resourceScaler, err := resourcescaler.New(rootLogger, namespace, nuclioClientSet, platformConfiguration)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to create resource scaler")
	}

	// get resource scaler configuration
	resourceScalerConfig, err := resourceScaler.GetConfig()
	if err != nil {
		return nil, errors.Wrap(err, "Failed to get resource scaler config")
	}

	// create dlx instance
	dlxInstance, err := dlx.NewDLX(rootLogger, resourceScaler, resourceScalerConfig.DLXOptions)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to create dlx instance")
	}
	return dlxInstance, nil
}
