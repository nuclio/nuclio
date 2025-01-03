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
	"github.com/nuclio/nuclio/pkg/common/healthcheck"
	"github.com/nuclio/nuclio/pkg/common/status"
	"github.com/nuclio/nuclio/pkg/platformconfig"

	"github.com/nuclio/errors"
	"github.com/nuclio/logger"
)

type ProcessorServer struct {
	*healthcheck.AbstractServer
}

func NewProcessorServer(logger logger.Logger,
	statusProvider status.Provider,
	configuration *platformconfig.WebServer) (*ProcessorServer, error) {
	var err error

	newServer := &ProcessorServer{}
	newServer.AbstractServer, err = healthcheck.NewAbstractServer(logger, statusProvider, configuration)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to create new abstract server")
	}
	return newServer, nil
}

func (s *ProcessorServer) Start() error {

	// if we're disabled, simply log and do nothing
	if !s.AbstractServer.Enabled {
		s.AbstractServer.Logger.Debug("Disabled, not listening")
		return nil
	}

	// register the processor's status check as its readiness check
	s.AbstractServer.Handler.AddReadinessCheck("processor_readiness", func() error {
		if s.AbstractServer.StatusProvider.GetStatus() != status.Ready {
			return errors.New("Processor not ready yet")
		}

		return nil
	})

	// register an always-healthy liveness check until we have a better design for detecting handler deaths
	s.AbstractServer.Handler.AddLivenessCheck("processor_liveness", func() error {
		return nil
	})

	if err := s.AbstractServer.Start(); err != nil {
		return errors.Wrap(err, "Failed to start healthcheck server")
	}

	return nil
}
