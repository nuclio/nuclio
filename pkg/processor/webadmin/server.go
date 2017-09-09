package webadmin

import (
	"net/http"

	"github.com/go-chi/chi"
	"github.com/go-chi/chi/middleware"
	"github.com/nuclio/nuclio-sdk"
	"github.com/pkg/errors"
	"github.com/spf13/viper"
)

type Server struct {
	logger        nuclio.Logger
	enabled       bool
	listenAddress string
	router        chi.Router
	processor     interface{}
}

type resourceInitializer interface {
	Initialize(nuclio.Logger, interface{}) (chi.Router, error)
}

func NewServer(parentLogger nuclio.Logger, processor interface{}, configuration *viper.Viper) (*Server, error) {
	var err error

	newServer := &Server{
		logger:    parentLogger.GetChild("webadmin").(nuclio.Logger),
		processor: processor,
	}

	err = newServer.readConfiguration(configuration)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to read configuration")
	}

	// if we're not enabled, we're done here
	if !newServer.enabled {
		return newServer, nil
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

	// if we're not enabled, we're done here
	if !s.enabled {
		s.logger.DebugWith("Disabled, not listening")

		return nil
	}

	go http.ListenAndServe(s.listenAddress, s.router)

	s.logger.InfoWith("Listening", "listenAddress", s.listenAddress)

	return nil
}

func (s *Server) readConfiguration(configuration *viper.Viper) error {

	// get function name
	if configuration == nil {

		// initialize with a new viper
		configuration = viper.New()
	}

	// by default web admin is enabled
	configuration.SetDefault("enabled", true)

	// by default web admin listens on port 8081
	configuration.SetDefault("listen_address", ":8081")

	// set configuration
	s.enabled = configuration.GetBool("enabled")
	s.listenAddress = configuration.GetString("listen_address")

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
