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

package dashboard

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/nuclio/nuclio/pkg/common"
	"github.com/nuclio/nuclio/pkg/dockerclient"
	"github.com/nuclio/nuclio/pkg/dockercreds"
	"github.com/nuclio/nuclio/pkg/errors"
	"github.com/nuclio/nuclio/pkg/platform"
	"github.com/nuclio/nuclio/pkg/platformconfig"
	"github.com/nuclio/nuclio/pkg/restful"

	"github.com/go-chi/chi"
	"github.com/go-chi/cors"
	"github.com/nuclio/logger"
)

type Server struct {
	*restful.AbstractServer
	dockerKeyDir          string
	defaultRegistryURL    string
	defaultRunRegistryURL string
	dockerClient          dockerclient.Client
	dockerCreds           *dockercreds.DockerCreds
	Platform              platform.Platform
	NoPullBaseImages      bool
	externalIPAddresses   []string
}

func NewServer(parentLogger logger.Logger,
	dockerKeyDir string,
	defaultRegistryURL string,
	defaultRunRegistryURL string,
	platform platform.Platform,
	noPullBaseImages bool,
	configuration *platformconfig.WebServer,
	defaultCredRefreshInterval *time.Duration,
	externalIPAddresses []string) (*Server, error) {

	var err error

	newDockerClient, err := dockerclient.NewShellClient(parentLogger, nil)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to create docker client")
	}

	newDockerCreds, err := dockercreds.NewDockerCreds(parentLogger, newDockerClient, defaultCredRefreshInterval)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to create docker loginner")
	}

	newServer := &Server{
		dockerKeyDir:          dockerKeyDir,
		defaultRegistryURL:    defaultRegistryURL,
		defaultRunRegistryURL: defaultRunRegistryURL,
		dockerClient:          newDockerClient,
		dockerCreds:           newDockerCreds,
		Platform:              platform,
		NoPullBaseImages:      noPullBaseImages,
		externalIPAddresses:   externalIPAddresses,
	}

	// create server
	newServer.AbstractServer, err = restful.NewAbstractServer(parentLogger,
		DashboardResourceRegistrySingleton,
		newServer,
		configuration)

	if err != nil {
		return nil, errors.Wrap(err, "Failed to create restful server")
	}

	// try to load docker keys, ignoring errors
	if err := newServer.loadDockerKeys(newServer.dockerKeyDir); err != nil {
		newServer.Logger.WarnWith("Failed to login with docker keys", "err", err.Error())
	}

	// if the docker registry was not specified, try to take from credentials. this way the user only needs
	// to specify the secret to that registry and URL will be taken from there
	if newServer.defaultRegistryURL == "" {
		newServer.defaultRegistryURL = newServer.getRegistryURL()
	}

	// for logging purposes, duration can't be nil (stringer is called on nil and panics)
	if defaultCredRefreshInterval == nil {
		noDefaultCredRefreshInterval := 0 * time.Second

		defaultCredRefreshInterval = &noDefaultCredRefreshInterval
	}

	// set external IPs, if specified
	if len(externalIPAddresses) != 0 {
		if err := newServer.Platform.SetExternalIPAddresses(externalIPAddresses); err != nil {
			return nil, errors.Wrap(err, "Failed to set external IP address")
		}
	}

	newServer.Logger.InfoWith("Initialized",
		"dockerKeyDir", dockerKeyDir,
		"defaultRegistryURL", defaultRegistryURL,
		"defaultRunRegistryURL", defaultRunRegistryURL,
		"defaultCredRefreshInterval", defaultCredRefreshInterval)

	return newServer, nil
}

func (s *Server) GetRegistryURL() string {
	return s.defaultRegistryURL
}

func (s *Server) GetRunRegistryURL() string {
	return s.defaultRunRegistryURL
}

func (s *Server) GetExternalIPAddresses() []string {
	return s.externalIPAddresses
}

func (s *Server) InstallMiddleware(router chi.Router) error {
	if err := s.AbstractServer.InstallMiddleware(router); err != nil {
		return err
	}

	headers := []string{
		"X-nuclio-log-level",
		"X-nuclio-function-name",
		"X-nuclio-function-namespace",
		"X-nuclio-wait-function-action",
		"X-nuclio-path",
		"X-nuclio-invoke-via",
		"X-nuclio-project-name",
		"X-nuclio-project-namespace",
		"X-nuclio-function-event-name",
		"X-nuclio-function-event-namespace",
		"X-nuclio-filter-contents",
	}

	corsOptions := cors.Options{
		AllowedOrigins: []string{"*"},
		AllowedMethods: []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders: []string{
			"Accept",
			"Authorization",
			"Content-Type",
			"X-CSRF-Token",
		},
		ExposedHeaders:   headers,
		AllowCredentials: true,
		MaxAge:           300,
	}

	// add headers to allowed headers
	corsOptions.AllowedHeaders = append(corsOptions.AllowedHeaders, headers...)

	// create new CORS instance
	router.Use(cors.New(corsOptions).Handler)

	return nil
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(201)
}

func (s *Server) getRegistryURL() string {
	registryURL := ""
	credentials := s.dockerCreds.GetCredentials()

	if len(credentials) >= 1 {
		registryURL = credentials[0].URL

		// if the user specified the docker hub, we can't use this as-is. add the user name to the URL
		// to generate a valid URL
		for _, dockerPattern := range []string{
			".docker.com",
			".docker.io",
		} {
			if strings.HasSuffix(registryURL, dockerPattern) {
				registryURL = fmt.Sprintf("%s/%s", registryURL, credentials[0].Username)
				break
			}
		}

		// trim prefixes
		registryURL = common.StripPrefixes(registryURL,
			[]string{
				"https://",
				"http://",
			})

		s.Logger.InfoWith("Using registry from credentials", "url", registryURL)
	}

	// if we're still without a valid registry, use a hardcoded one (TODO: remove this)
	if registryURL == "" {
		registryURL = "localhost:5000"
	}

	return registryURL
}

func (s *Server) loadDockerKeys(dockerKeyDir string) error {
	if dockerKeyDir == "" {
		return nil
	}

	return s.dockerCreds.LoadFromDir(dockerKeyDir)
}
