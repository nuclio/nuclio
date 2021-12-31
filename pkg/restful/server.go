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

package restful

import (
	"net/http"

	"github.com/nuclio/nuclio/pkg/platformconfig"
	"github.com/nuclio/nuclio/pkg/registry"
	nucliomiddleware "github.com/nuclio/nuclio/pkg/restful/middleware"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/nuclio/errors"
	"github.com/nuclio/logger"
)

type Server interface {

	// InstallMiddleware installs middlewares on a router
	InstallMiddleware(router chi.Router) error

	// Start running the server
	Start() error
}

type AbstractServer struct {
	Logger           logger.Logger
	Enabled          bool
	ListenAddress    string
	Router           chi.Router
	resourceRegistry *registry.Registry
	server           Server
}

func NewAbstractServer(parentLogger logger.Logger,
	resourceRegistry *registry.Registry,
	server Server,
	configuration *platformconfig.WebServer) (*AbstractServer, error) {

	var err error

	newServer := &AbstractServer{
		Logger:           parentLogger.GetChild("server"),
		resourceRegistry: resourceRegistry,
		server:           server,
	}

	newServer.Router, err = newServer.createRouter()
	if err != nil {
		return nil, errors.Wrap(err, "Failed to create router")
	}

	// install the middleware
	if err := server.InstallMiddleware(newServer.Router); err != nil {
		return nil, errors.Wrap(err, "Failed to install middleware")
	}

	if err := newServer.readConfiguration(configuration); err != nil {
		return nil, errors.Wrap(err, "Failed to read configuration")
	}

	// create the resources registered
	for _, resourceName := range newServer.resourceRegistry.GetKinds() {
		resolvedResource, _ := newServer.resourceRegistry.Get(resourceName)
		resourceInstance := resolvedResource.(Resource)

		// create the resource router and add it
		resourceRouter, err := resourceInstance.Initialize(newServer.Logger, newServer.server)
		if err != nil {
			return nil, errors.Wrapf(err, "Failed to create resource router for %s", resourceName)
		}

		// register the router into the root router
		newServer.Router.Mount("/"+resourceName, resourceRouter)

		newServer.Logger.DebugWith("Registered resource", "name", resourceName)
	}

	return newServer, nil
}

func (s *AbstractServer) Start() error {

	// if we're not enabled, we're done here
	if !s.Enabled {
		s.Logger.Debug("AbstractServer disabled, not listening")
		return nil
	}

	go http.ListenAndServe(s.ListenAddress, s.Router) // nolint: errcheck

	s.Logger.InfoWith("Listening", "listenAddress", s.ListenAddress)

	return nil
}

func (s *AbstractServer) InstallMiddleware(router chi.Router) error {
	router.Use(nucliomiddleware.RequestResponseLogger(s.Logger))
	router.Use(middleware.Recoverer)
	router.Use(middleware.StripSlashes)
	router.Use(nucliomiddleware.RequestID)
	router.Use(nucliomiddleware.AlignRequestIDKeyToZapLogger)
	return nil
}

func (s *AbstractServer) createRouter() (chi.Router, error) {
	router := chi.NewRouter()

	if err := s.InstallMiddleware(router); err != nil {
		return nil, errors.Wrap(err, "Failed to install middleware")
	}

	return router, nil
}

func (s *AbstractServer) readConfiguration(configuration *platformconfig.WebServer) error {
	if configuration.Enabled == nil {
		return errors.New("Enabled must carry a value")
	}

	// set configuration
	s.Enabled = *configuration.Enabled
	s.ListenAddress = configuration.ListenAddress

	return nil
}
