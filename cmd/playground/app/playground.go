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
	"github.com/nuclio/nuclio/pkg/playground"

	"github.com/nuclio/nuclio/pkg/zap"
	"github.com/pkg/errors"
)

func Run() error {

	logger, err := nucliozap.NewNuclioZapCmd("playground", nucliozap.DebugLevel)
	if err != nil {
		return errors.Wrap(err, "Failed to create logger")
	}

	server, err := playground.NewServer(logger, "/Users/erand/Development/iguazio/nuclio/src/github.com/nuclio/nuclio/pkg/playground/assets")
	if err != nil {
		return errors.Wrap(err, "Failed to create server")
	}

	server.Enabled = true
	server.ListenAddress = ":8082"

	err = server.Start()
	if err != nil {
		return errors.Wrap(err, "Failed to start server")
	}

	select {}

	return nil
}
