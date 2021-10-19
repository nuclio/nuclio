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
	"github.com/v3io/scaler/pkg/autoscaler"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/discovery/cached/memory"
	"k8s.io/client-go/restmapper"
	"k8s.io/metrics/pkg/client/custom_metrics"
)

func Run(platformConfigurationPath string, namespace string, kubeconfigPath string) error {

	// create autoscaler
	autoScaler, err := createAutoScaler(platformConfigurationPath, namespace, kubeconfigPath)
	if err != nil {
		return errors.Wrap(err, "Failed to create autoscaler")
	}

	// start autoscaler and run forever
	if err := autoScaler.Start(); err != nil {
		return errors.Wrap(err, "Failed to start autoscaler")
	}
	select {}
}

func createAutoScaler(platformConfigurationPath string,
	namespace string,
	kubeconfigPath string) (*autoscaler.Autoscaler, error) {

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

	// create k8s rest config
	customMetricsClient, err := newMetricsCustomClient(kubeconfigPath)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to create new metric custom client")
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

	// create autoscaler
	autoScaler, err := autoscaler.NewAutoScaler(rootLogger,
		resourceScaler,
		customMetricsClient,
		resourceScalerConfig.AutoScalerOptions)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to create autoscaler")
	}

	return autoScaler, nil
}

func newMetricsCustomClient(kubeconfigPath string) (custom_metrics.CustomMetricsClient, error) {
	restConfig, err := common.GetClientConfig(kubeconfigPath)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to get rest config")
	}

	// create metric client and
	discoveryClient, err := discovery.NewDiscoveryClientForConfig(restConfig)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to create discovery client")
	}
	availableAPIsGetter := custom_metrics.NewAvailableAPIsGetter(discoveryClient)
	restMapper := restmapper.NewDeferredDiscoveryRESTMapper(memory.NewMemCacheClient(discoveryClient))
	return custom_metrics.NewForConfig(restConfig, restMapper, availableAPIsGetter), nil
}
