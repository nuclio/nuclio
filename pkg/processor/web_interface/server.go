package web_interface

import (
	"net/http"

	"github.com/go-chi/chi"
	"github.com/go-chi/chi/middleware"
	"github.com/nuclio/nuclio-sdk"
	"github.com/pkg/errors"
)

type Server struct {
	logger        nuclio.Logger
	listenAddress string
	router        chi.Router
	processor     interface{}
}

type resourceInitializer interface {
	Initialize(nuclio.Logger, interface{}) (chi.Router, error)
}

func NewServer(parentLogger nuclio.Logger, processor interface{}) (*Server, error) {
	var err error

	newServer := &Server{
		logger:        parentLogger.GetChild("webif").(nuclio.Logger),
		listenAddress: ":8081",
		processor:     processor,
	}

	newServer.router, err = newServer.createRouter()
	if err != nil {
		return nil, errors.Wrap(err, "Failed to create router")
	}

	// create the resources registered
	for _, resourceName := range ResourceRegistrySingleton.GetKinds() {
		resourceInstance, _ := ResourceRegistrySingleton.Get(resourceName)

		// create the resource router and add it
		resourceRouter, err := resourceInstance.(resourceInitializer).Initialize(newServer.logger, processor)
		if err != nil {
			return nil, errors.Wrapf(err, "Failed to create resource router for %s", resourceName)
		}

		// register the router into the root router
		newServer.router.Mount("/"+resourceName, resourceRouter)

		newServer.logger.DebugWith("Registered resource", "name", resourceName)
	}

	return newServer, nil
}

func (s *Server) Start() error {
	go http.ListenAndServe(s.listenAddress, s.router)

	s.logger.InfoWith("Listening", "address", s.listenAddress)

	return nil
}

func (s *Server) createRouter() (chi.Router, error) {
	router := chi.NewRouter()

	router.Use(middleware.Recoverer)
	router.Use(middleware.StripSlashes)
	router.Use(setContentType)

	return router, nil
}

// SetCtype is a middleware that set content type to JSON API content type
func setContentType(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/vnd.api+json")
		next.ServeHTTP(w, r)
	})
}
