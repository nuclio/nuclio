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
	"github.com/nuclio/nuclio/pkg/restful"

	"github.com/nuclio/logger"
)

type Server struct {
	*restful.Server
	Processor interface{}
}

func NewServer(parentLogger logger.Logger, processor interface{}, configuration *platformconfig.WebServer) (*Server, error) {
	var err error

	newServer := &Server{Processor: processor}

	// namespace our logger
	logger := parentLogger.GetChild("webadmin")

	// create server
	newServer.Server, err = restful.NewServer(logger, WebAdminResourceRegistrySingleton, newServer, configuration)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to create restful server")
	}

	return newServer, nil
}
