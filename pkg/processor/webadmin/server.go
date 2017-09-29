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
	"github.com/nuclio/nuclio/pkg/restful"

	"github.com/nuclio/nuclio-sdk"
	"github.com/spf13/viper"
)

type Server struct {
	*restful.Server
	Processor interface{}
}

func NewServer(parentLogger nuclio.Logger, processor interface{}, configuration *viper.Viper) (*Server, error) {
	var err error

	newServer := &Server{Processor: processor}

	// create server
	newServer.Server, err = restful.NewServer(parentLogger, WebAdminResourceRegistrySingleton, newServer)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to create restful server")
	}

	err = newServer.readConfiguration(configuration)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to read configuration")
	}

	return newServer, nil
}

func (s *Server) readConfiguration(configuration *viper.Viper) error {

	// by default web admin is enabled
	configuration.SetDefault("enabled", true)

	// by default web admin listens on port 8081
	configuration.SetDefault("listen_address", ":8081")

	// set configuration
	s.Enabled = configuration.GetBool("enabled")
	s.ListenAddress = configuration.GetString("listen_address")

	return nil
}
