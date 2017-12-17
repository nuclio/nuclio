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

	"github.com/nuclio/nuclio/pkg/errors"
	"github.com/nuclio/nuclio/pkg/registry"

	"github.com/go-chi/chi"
	"github.com/go-chi/chi/middleware"
	"github.com/nuclio/nuclio-sdk"
)

type Server struct {
	Logger           nuclio.Logger
	Enabled          bool
	ListenAddress    string
	Router           chi.Router
	resourceRegistry *registry.Registry
	conreteServer    interface{}
}

type resourceInitializer interface {
	Initialize(nuclio.Logger, interface{}) (chi.Router, error)
}

func NewServer(parentLogger nuclio.Logger,
	resourceRegistry *registry.Registry,
	conreteServer interface{}) (*Server, error) {

	var err error

	newServer := &Server{
		Logger:           parentLogger.GetChild("server"),
		resourceRegistry: resourceRegistry,
		conreteServer:    conreteServer,
	}

	newServer.Router, err = newServer.createRouter()
	if err != nil {
		return nil, errors.Wrap(err, "Failed to create router")
	}

	return newServer, nil
}

func (s *Server) Start() error {

	// if we're not enabled, we're done here
	if !s.Enabled {
		return nil
	}

	// create the resources registered
	for _, resourceName := range s.resourceRegistry.GetKinds() {
		resourceInstance, _ := s.resourceRegistry.Get(resourceName)

		// create the resource router and add it
		resourceRouter, err := resourceInstance.(resourceInitializer).Initialize(s.Logger, s.conreteServer)
		if err != nil {
			return errors.Wrapf(err, "Failed to create resource router for %s", resourceName)
		}

		// register the router into the root router
		s.Router.Mount("/"+resourceName, resourceRouter)

		s.Logger.DebugWith("Registered resource", "name", resourceName)
	}

	// if we're not enabled, we're done here
	if !s.Enabled {
		s.Logger.DebugWith("Disabled, not listening")

		return nil
	}

	go http.ListenAndServe(s.ListenAddress, s.Router)

	s.Logger.InfoWith("Listening", "listenAddress", s.ListenAddress)

	return nil
}

func (s *Server) InstallMiddleware(router chi.Router) error {
	router.Use(middleware.Recoverer)
	router.Use(middleware.StripSlashes)
	router.Use(setCORSOrigin)

	return nil
}

func (s *Server) createRouter() (chi.Router, error) {
	router := chi.NewRouter()

	s.InstallMiddleware(router)

	return router, nil
}

// middleware that sets content type to JSON content type
func setCORSOrigin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		next.ServeHTTP(w, r)
	})
}
