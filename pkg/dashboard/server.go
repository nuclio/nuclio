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
	"time"

	"github.com/nuclio/nuclio/pkg/common"
	"github.com/nuclio/nuclio/pkg/dashboard/auth"
	authfactory "github.com/nuclio/nuclio/pkg/dashboard/auth/factory"
	"github.com/nuclio/nuclio/pkg/dashboard/functiontemplates"
	"github.com/nuclio/nuclio/pkg/dockerclient"
	"github.com/nuclio/nuclio/pkg/dockercreds"
	"github.com/nuclio/nuclio/pkg/platform"
	"github.com/nuclio/nuclio/pkg/platform/abstract/project/external/leader/iguazio"
	"github.com/nuclio/nuclio/pkg/platformconfig"
	"github.com/nuclio/nuclio/pkg/restful"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/cors"
	"github.com/nuclio/errors"
	"github.com/nuclio/logger"
)

type PlatformAuthorizationMode string

const (
	PlatformAuthorizationModeServiceAccount          PlatformAuthorizationMode = "service-account"
	PlatformAuthorizationModeAuthorizationHeaderOIDC PlatformAuthorizationMode = "authorization-header-oidc"
)

type Server struct {
	*restful.AbstractServer
	dockerKeyDir              string
	defaultRegistryURL        string
	defaultRunRegistryURL     string
	dockerCreds               *dockercreds.DockerCreds
	Platform                  platform.Platform
	NoPullBaseImages          bool
	externalIPAddresses       []string
	defaultNamespace          string
	Offline                   bool
	Repository                *functiontemplates.Repository
	platformConfiguration     *platformconfig.Config
	imageNamePrefixTemplate   string
	platformAuthorizationMode PlatformAuthorizationMode
	dependantImageRegistryURL string

	// auth options
	authInstance auth.Auth
}

func NewServer(parentLogger logger.Logger,
	containerBuilderKind string,
	dockerKeyDir string,
	defaultRegistryURL string,
	defaultRunRegistryURL string,
	platform platform.Platform,
	noPullBaseImages bool,
	configuration *platformconfig.WebServer,
	defaultCredRefreshInterval *time.Duration,
	externalIPAddresses []string,
	defaultNamespace string,
	offline bool,
	repository *functiontemplates.Repository,
	platformConfiguration *platformconfig.Config,
	imageNamePrefixTemplate string,
	platformAuthorizationMode string,
	dependantImageRegistryURL string,
	authConfig *auth.Config) (*Server, error) {

	// newDockerClient may be nil
	newDockerClient, err := createDockerClient(parentLogger, containerBuilderKind)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to create docker client")
	}

	newDockerCreds, err := dockercreds.NewDockerCreds(parentLogger, newDockerClient, defaultCredRefreshInterval)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to create docker loginner")
	}

	// if we're set to build offline, make sure not to pull base images
	if offline {
		noPullBaseImages = true
	}

	newServer := &Server{
		dockerKeyDir:              dockerKeyDir,
		defaultRegistryURL:        defaultRegistryURL,
		defaultRunRegistryURL:     defaultRunRegistryURL,
		dockerCreds:               newDockerCreds,
		Platform:                  platform,
		NoPullBaseImages:          noPullBaseImages,
		externalIPAddresses:       externalIPAddresses,
		defaultNamespace:          defaultNamespace,
		Offline:                   offline,
		Repository:                repository,
		platformConfiguration:     platformConfiguration,
		imageNamePrefixTemplate:   imageNamePrefixTemplate,
		platformAuthorizationMode: PlatformAuthorizationMode(platformAuthorizationMode),
		dependantImageRegistryURL: dependantImageRegistryURL,
		authInstance:              authfactory.NewAuth(parentLogger, authConfig),
	}

	// create server
	newServer.AbstractServer, err = restful.NewAbstractServer(parentLogger,
		DashboardResourceRegistrySingleton,
		newServer)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to create restful server")
	}

	if err := newServer.Initialize(configuration); err != nil {
		return nil, errors.Wrap(err, "Failed to initialize new server")
	}

	// try to load docker keys, ignoring errors
	if containerBuilderKind == "docker" {
		if err := newServer.loadDockerKeys(newServer.dockerKeyDir); err != nil {
			newServer.Logger.WarnWith("Failed to login with docker keys", "err", err.Error())
		}
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
		"defaultCredRefreshInterval", defaultCredRefreshInterval,
		"defaultNamespace", defaultNamespace)

	return newServer, nil
}

func (s *Server) GetRegistryURL() string {
	return s.defaultRegistryURL
}

func (s *Server) GetRunRegistryURL() string {
	return s.defaultRunRegistryURL
}

func (s *Server) GetDependantImagesRegistryURL() string {
	return s.dependantImageRegistryURL
}

func (s *Server) GetExternalIPAddresses() []string {
	return s.externalIPAddresses
}

func (s *Server) GetImageNamePrefixTemplate() string {
	return s.imageNamePrefixTemplate
}

func (s *Server) GetDefaultNamespace() string {
	return s.defaultNamespace
}

func (s *Server) GetPlatformConfiguration() *platformconfig.Config {
	return s.platformConfiguration
}

func (s *Server) InstallMiddleware(router chi.Router) error {
	if err := s.AbstractServer.InstallMiddleware(router); err != nil {
		return errors.Wrap(err, "Failed to install abstract server router middleware")
	}

	corsOptions := cors.Options{
		AllowedOrigins: []string{"*"},
		AllowedMethods: []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders: []string{
			"Accept",
			"Authorization",
			"Content-Type",
			"Content-Length",
			"X-CSRF-Token",
			"X-nuclio-log-level",
			"X-nuclio-function-name",
			"X-nuclio-function-namespace",
			"X-nuclio-wait-function-action",
			"X-nuclio-api-gateway-name",
			"X-nuclio-api-gateway-namespace",
			"X-nuclio-invoke-timeout",
			"X-nuclio-invoke-via",
			"X-nuclio-invoke-url",
			"X-nuclio-project-name",
			"X-nuclio-project-namespace",
			"X-nuclio-function-event-name",
			"X-nuclio-function-event-namespace",
			"X-nuclio-function-enrich-apigateways",
			"X-nuclio-path",
			"x-nuclio-filter-contains",
			"X-nuclio-delete-project-strategy",
			"X-nuclio-delete-function-ignore-state-validation",
			"X-nuclio-agw-validate-functions-existence",
			iguazio.ProjectsRoleHeaderKey,
		},
		ExposedHeaders: []string{
			"Content-Length",
			"X-nuclio-logs",
		},
		AllowCredentials: true,
		MaxAge:           300,
	}

	// create new CORS instance
	router.Use(cors.New(corsOptions).Handler)

	return nil
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(201)
}

func (s *Server) GetPlatformAuthorizationMode() PlatformAuthorizationMode {
	return s.platformAuthorizationMode
}

func (s *Server) GetAuthenticator() auth.Auth {
	return s.authInstance
}

func (s *Server) getRegistryURL() string {
	registryURL := ""
	credentials := s.dockerCreds.GetCredentials()

	if len(credentials) >= 1 {
		registryURL = s.resolveDockerCredentialsRegistryURL(credentials[0])
		s.Logger.InfoWith("Using registry from credentials", "url", registryURL)
	}

	return registryURL
}

func (s *Server) resolveDockerCredentialsRegistryURL(credentials dockercreds.Credentials) string {
	registryURL := credentials.URL

	// TODO: This auto-expansion does not support with kaniko today, must provide full URL. Remove this?
	// if the user specified the docker hub, we can't use this as-is. add the user name to the URL
	// to generate a valid URL
	if common.MatchStringPatterns([]string{
		`\.docker\.com`,
		`\.docker\.io`,
	}, registryURL) {
		registryURL = common.StripSuffixes(registryURL, []string{

			// when using docker.io as login address, the resolved address in the docker credentials file
			// might contain the registry version, strip it if so
			"/v1",
			"/v1/",
		})
		registryURL = fmt.Sprintf("%s/%s", registryURL, credentials.Username)
	}

	// trim prefixes
	registryURL = common.StripPrefixes(registryURL,
		[]string{
			"https://",
			"http://",
		})
	return registryURL
}

func (s *Server) loadDockerKeys(dockerKeyDir string) error {
	if dockerKeyDir == "" {
		return nil
	}

	return s.dockerCreds.LoadFromDir(dockerKeyDir)
}

func createDockerClient(parentLogger logger.Logger, containerBuilderKind string) (
	dockerclient.Client, error) {
	if containerBuilderKind == "docker" {
		return dockerclient.NewShellClient(parentLogger, nil)
	}

	// if docker won't be use, return nil as a client
	return nil, nil
}
