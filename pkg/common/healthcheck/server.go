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

package healthcheck

import (
	"net/http"

	"github.com/nuclio/nuclio/pkg/common/status"
	"github.com/nuclio/nuclio/pkg/platformconfig"

	"github.com/heptiolabs/healthcheck"
	"github.com/nuclio/errors"
	"github.com/nuclio/logger"
)

type Server interface {

	// Start the server
	Start() error
}

type AbstractServer struct {
	Enabled        bool
	ListenAddress  string
	Logger         logger.Logger
	StatusProvider status.Provider
	Handler        healthcheck.Handler
}

func NewAbstractServer(logger logger.Logger,
	statusProvider status.Provider,
	configuration *platformconfig.WebServer) (*AbstractServer, error) {
	if configuration.Enabled == nil {
		return nil, errors.New("Enabled must carry a value")
	}

	server := &AbstractServer{
		Enabled:        *configuration.Enabled,
		ListenAddress:  configuration.ListenAddress,
		Logger:         logger.GetChild("healthcheck.server"),
		StatusProvider: statusProvider,
	}

	// create the healthcheck handler
	server.Handler = healthcheck.NewHandler()

	// return the server
	return server, nil
}

func (s *AbstractServer) Start() error {

	// start listening
	go http.ListenAndServe(s.ListenAddress, s.Handler) // nolint: errcheck

	s.Logger.InfoWith("Listening", "listenAddress", s.ListenAddress)
	return nil
}
