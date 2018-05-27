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

package webadmin

import (
	"github.com/nuclio/nuclio/pkg/errors"
	"github.com/nuclio/nuclio/pkg/platformconfig"
	"github.com/nuclio/nuclio/pkg/processor/webadmin/dealer"
	"github.com/nuclio/nuclio/pkg/restful"

	"github.com/nuclio/logger"
)

// Server is the webadmin web server
type Server struct {
	*restful.AbstractServer
	Processor interface{}
}

// NewServer returns new webadmin server
func NewServer(parentLogger logger.Logger, processor interface{}, configuration *platformconfig.WebServer) (*Server, error) {
	var err error

	newServer := &Server{Processor: processor}

	// namespace our logger
	logger := parentLogger.GetChild("webadmin")

	// create server
	newServer.AbstractServer, err = restful.NewAbstractServer(logger,
		WebAdminResourceRegistrySingleton,
		newServer,
		configuration)

	if err != nil {
		return nil, errors.Wrap(err, "Failed to create restful server")
	}

	err = newServer.createDealer(logger, processor, configuration)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to create dealer API")
	}

	return newServer, nil
}

func (s *Server) createDealer(logger logger.Logger, processor interface{}, configuration *platformconfig.WebServer) error {
	// Dealer doesn't fit in restul
	dealer, err := dealer.New(logger, processor, configuration)
	if err != nil {
		return errors.Wrap(err, "Can't create dealer")
	}
	s.Router.Get("/dealer", dealer.Get)
	s.Router.Post("/dealer", dealer.Post)

	return nil
}
