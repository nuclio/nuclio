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
	"strings"
	"time"

	"github.com/nuclio/nuclio/pkg/dashboard"
	"github.com/nuclio/nuclio/pkg/errors"
	"github.com/nuclio/nuclio/pkg/platform/factory"
	"github.com/nuclio/nuclio/pkg/platformconfig"
	"github.com/nuclio/nuclio/pkg/version"

	"github.com/nuclio/zap"
)

func Run(listenAddress string,
	dockerKeyDir string,
	defaultRegistryURL string,
	defaultRunRegistryURL string,
	platformType string,
	noPullBaseImages bool,
	defaultCredRefreshIntervalString string,
	externalIPAddresses string,
	defaultNamespace string,
	offline bool,
	platformConfigurationPath string) error {

	logger, err := nucliozap.NewNuclioZapCmd("dashboard", nucliozap.DebugLevel)
	if err != nil {
		return errors.Wrap(err, "Failed to create logger")
	}

	// create a platform
	platformInstance, err := factory.CreatePlatform(logger, platformType, nil)
	if err != nil {
		return errors.Wrap(err, "Failed to create platform")
	}

	// set external ip addresses based if user passed overriding values or not
	var splitExternalIPAddresses []string
	if externalIPAddresses == "" {
		splitExternalIPAddresses, err = platformInstance.GetDefaultInvokeIPAddresses()
		if err != nil {
			return errors.Wrap(err, "Failed to get default invoke ip addresses")
		}
	} else {

		// "10.0.0.1,10.0.0.2" -> ["10.0.0.1", "10.0.0.2"]
		splitExternalIPAddresses = strings.Split(externalIPAddresses, ",")
	}

	err = platformInstance.SetExternalIPAddresses(splitExternalIPAddresses)
	if err != nil {
		return errors.Wrap(err, "Failed to set external ip addresses")
	}

	logger.InfoWith("Starting",
		"name", platformInstance.GetName(),
		"noPull", noPullBaseImages,
		"offline", offline,
		"defaultCredRefreshInterval", defaultCredRefreshIntervalString,
		"defaultNamespace", defaultNamespace,
		"platformConfigurationPath", platformConfigurationPath)

	// see if the platform has anything to say about the namespace
	defaultNamespace = platformInstance.ResolveDefaultNamespace(defaultNamespace)

	version.Log(logger)

	trueValue := true

	// create a web server configuration
	webServerConfiguration := &platformconfig.WebServer{
		Enabled:       &trueValue,
		ListenAddress: listenAddress,
	}

	server, err := dashboard.NewServer(logger,
		dockerKeyDir,
		defaultRegistryURL,
		defaultRunRegistryURL,
		platformInstance,
		noPullBaseImages,
		webServerConfiguration,
		getDefaultCredRefreshInterval(logger, defaultCredRefreshIntervalString),
		splitExternalIPAddresses,
		defaultNamespace,
		offline,
		platformConfigurationPath)
	if err != nil {
		return errors.Wrap(err, "Failed to create server")
	}

	err = server.Start()
	if err != nil {
		return errors.Wrap(err, "Failed to start server")
	}

	select {}
}

func getDefaultCredRefreshInterval(logger *nucliozap.NuclioZap, defaultCredRefreshIntervalString string) *time.Duration {
	var defaultCredRefreshInterval time.Duration
	defaultInterval := 12 * time.Hour

	// if set to "none" - no refresh interval
	if defaultCredRefreshIntervalString == "none" {
		return nil
	}

	// if unspecified, default to 12 hours
	if defaultCredRefreshIntervalString == "" {
		return &defaultInterval
	}

	// try to parse the refresh interval - if failed
	defaultCredRefreshInterval, err := time.ParseDuration(defaultCredRefreshIntervalString)
	if err != nil {
		logger.WarnWith("Failed to parse default credential refresh interval, defaulting",
			"given", defaultCredRefreshIntervalString,
			"default", defaultInterval)

		return &defaultInterval
	}

	return &defaultCredRefreshInterval
}
