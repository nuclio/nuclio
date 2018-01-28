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

package healthcheck

import (
	"net/http"

	"github.com/nuclio/nuclio/pkg/errors"
	"github.com/nuclio/nuclio/pkg/platformconfig"
	"github.com/nuclio/nuclio/pkg/processor/status"

	"github.com/heptiolabs/healthcheck"
	"github.com/nuclio/logger"
)

type statusChecker func() status.Status

type Server struct {
	Enabled              bool
	ListenAddress        string
	logger               logger.Logger
	processorStatusCheck statusChecker
	handler              healthcheck.Handler
}

func NewServer(logger logger.Logger, processorStatusCheck statusChecker, configuration *platformconfig.WebServer) (*Server, error) {
	newServer := &Server{
		Enabled:              configuration.Enabled,
		ListenAddress:        configuration.ListenAddress,
		logger:               logger.GetChild("healthcheck.server"),
		processorStatusCheck: processorStatusCheck,
	}

	// create the healthcheck handler
	newServer.handler = healthcheck.NewHandler()

	// return the server
	return newServer, nil
}

func (s *Server) Start() error {

	// if we're disabled, simply log and do nothing
	if !s.Enabled {
		s.logger.Debug("Disabled, not listening")

		return nil
	}

	// register the processor's status check as both liveness and readiness checks
	// TODO: differ between them
	check := func() error {
		if s.processorStatusCheck() != status.Ready {
			return errors.New("Processor not ready yet")
		}

		return nil
	}

	s.handler.AddLivenessCheck("processor_status", check)
	s.handler.AddReadinessCheck("processor_status", check)

	// start listening
	go http.ListenAndServe(s.ListenAddress, s.handler)

	s.logger.InfoWith("Listening", "listenAddress", s.ListenAddress)

	return nil
}
