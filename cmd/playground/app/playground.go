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
	"github.com/nuclio/nuclio/pkg/errors"
	"github.com/nuclio/nuclio/pkg/platform/factory"
	"github.com/nuclio/nuclio/pkg/playground"
	"github.com/nuclio/nuclio/pkg/version"
	"github.com/nuclio/nuclio/pkg/zap"
)

func Run(listenAddress string,
	assetsDir string,
	sourcesDir string,
	dockerKeyDir string,
	defaultRegistryURL string,
	defaultRunRegistryURL string,
	platformType string,
	noPullBaseImages bool) error {

	logger, err := nucliozap.NewNuclioZapCmd("playground", nucliozap.DebugLevel)
	if err != nil {
		return errors.Wrap(err, "Failed to create logger")
	}

	// create a platform
	platformInstance, err := factory.CreatePlatform(logger, platformType, nil)
	if err != nil {
		return errors.Wrap(err, "Failed to create platform")
	}

	logger.InfoWith("Starting", "name", platformInstance.GetName(), "noPull", noPullBaseImages)

	version.Log(logger)

	server, err := playground.NewServer(logger,
		assetsDir,
		sourcesDir,
		dockerKeyDir,
		defaultRegistryURL,
		defaultRunRegistryURL,
		platformInstance,
		noPullBaseImages)
	if err != nil {
		return errors.Wrap(err, "Failed to create server")
	}

	server.Enabled = true
	server.ListenAddress = listenAddress

	err = server.Start()
	if err != nil {
		return errors.Wrap(err, "Failed to start server")
	}

	select {}
}
