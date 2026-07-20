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

package dashboard

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/nuclio/nuclio/pkg/auth"
	authfactory "github.com/nuclio/nuclio/pkg/auth/factory"
	"github.com/nuclio/nuclio/pkg/common"
	"github.com/nuclio/nuclio/pkg/common/headers"
	"github.com/nuclio/nuclio/pkg/dashboard/functiontemplates"
	"github.com/nuclio/nuclio/pkg/dockerclient"
	"github.com/nuclio/nuclio/pkg/dockercreds"
	"github.com/nuclio/nuclio/pkg/functionconfig"
	"github.com/nuclio/nuclio/pkg/platform"
	"github.com/nuclio/nuclio/pkg/platformconfig"
	"github.com/nuclio/nuclio/pkg/restful"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/cors"
	"github.com/nuclio/errors"
	"github.com/nuclio/logger"
	"github.com/nuclio/opa-client"
	"golang.org/x/sync/semaphore"
)

type PlatformAuthorizationMode string

const (
	PlatformAuthorizationModeServiceAccount          PlatformAuthorizationMode = "service-account"
	PlatformAuthorizationModeAuthorizationHeaderOIDC PlatformAuthorizationMode = "authorization-header-oidc"
	// maximum number of stale-function updates to run concurrently
	staleFunctionUpdateConcurrency = 5
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
	switch containerBuilderKind {
	case "docker":
		if err := newServer.loadDockerKeys(newServer.dockerKeyDir); err != nil {
			newServer.Logger.WarnWith("Failed to login with docker keys", "err", err.Error())
		}
	case "kaniko":
		if common.GetEnvOrDefaultStringWithLegacyKey("NUCLIO_DASHBOARD_SERVE_BUILD_ARTIFACTS_MODE",
			"NUCLIO_DASHBOARD_SERVE_KANIKO_ARTIFACTS_MODE", "local") == common.LocalPlatformName {

			// allow dashboard server to handle request to get kaniko artifacts for function builds
			// this is useful when running dashboard locally. in production, nginx will handle this
			newServer.Router.HandleFunc("/kaniko/*", func(w http.ResponseWriter, r *http.Request) {
				ctx := chi.RouteContext(r.Context())
				serverRoutePrefix := strings.TrimSuffix(ctx.RoutePattern(), "/*")
				fs := http.StripPrefix(serverRoutePrefix, http.FileServer(http.Dir("/tmp/kaniko-builds")))
				fs.ServeHTTP(w, r)
			})
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

// Start Starts the server and launches background job to mark stale functions as errored
func (s *Server) Start() error {
	if err := s.AbstractServer.Start(); err != nil {
		return errors.Wrap(err, "Failed to start server")
	}

	go s.markStaleFunctionsAsError(context.Background())

	return nil
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
		AllowedMethods: []string{"GET", "POST", "PUT", "DELETE", "OPTIONS", "PATCH"},
		AllowedHeaders: []string{
			"Accept",
			"Authorization",
			"Content-Type",
			"Content-Length",
			"X-CSRF-Token",
			headers.LogLevel,
			headers.FunctionName,
			headers.FunctionNamespace,
			headers.WaitFunctionAction,
			headers.ApiGatewayName,
			headers.ApiGatewayNamespace,
			headers.InvokeTimeout,
			headers.InvokeVia,
			headers.InvokeURL,
			headers.ProjectName,
			headers.ProjectNamespace,
			headers.FunctionEventName,
			headers.FunctionEventNamespace,
			headers.FunctionEnrichApiGateways,
			headers.Path,
			headers.FilterContains,
			headers.DeleteProjectStrategy,
			headers.DeleteFunctionIgnoreStateValidation,
			headers.ApiGatewayValidateFunctionExistence,
			headers.CreationStateUpdatedTimeout,
			headers.ProjectsRole,
		},
		ExposedHeaders: append(
			[]string{"Content-Length"},
			headers.GetAllowedResponseHeaderNames()...,
		),
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

// markStaleFunctionsAsError flips functions that are stuck in a pre-build/build state to error.
func (s *Server) markStaleFunctionsAsError(ctx context.Context) {
	functions, err := s.Platform.GetFunctions(ctx, &platform.GetFunctionsOptions{
		Namespace: s.GetDefaultNamespace(),
	})
	if err != nil {
		// non-fatal: a transient list failure shouldn't block the dashboard from starting
		s.Logger.WarnWithCtx(ctx, "Failed to list functions for stale-function sweep; skipping",
			"err", err.Error())
		return
	}

	// states from which no process can advance the function after a dashboard restart
	staleStates := map[functionconfig.FunctionState]struct{}{
		functionconfig.FunctionStateWaitingForBuild: {},
		functionconfig.FunctionStateBuilding:        {},
	}

	// update stale functions concurrently, bounded by a semaphore
	var wg sync.WaitGroup
	sem := semaphore.NewWeighted(staleFunctionUpdateConcurrency)

	for _, function := range functions {
		functionStatus := function.GetStatus()
		if _, isStale := staleStates[functionStatus.State]; !isStale {
			continue
		}

		functionConfig := function.GetConfig()
		s.Logger.DebugWithCtx(ctx, "Found stale function on startup, setting its state to error",
			"functionName", functionConfig.Meta.Name,
			"functionState", functionStatus.State)

		_ = sem.Acquire(ctx, 1)
		wg.Go(func() {
			defer sem.Release(1)

			functionStatus.State = functionconfig.FunctionStateError
			functionStatus.Message = "Function deployment was interrupted and could not be completed. Please redeploy the function."

			if err := s.Platform.UpdateFunction(ctx, &platform.UpdateFunctionOptions{
				FunctionMeta:   &functionConfig.Meta,
				FunctionSpec:   &functionConfig.Spec,
				FunctionStatus: functionStatus,

				// the sweep runs at startup with no user session; mark it as a system call so it
				// bypasses OPA authorization (the configured override value is matched by the OPA client)
				PermissionOptions: opaclient.PermissionOptions{
					OverrideHeaderValue: s.platformConfiguration.Opa.OverrideHeaderValue,
				},
			}); err != nil {

				// the kube platform's UpdateFunction persists the status and then waits for function
				// readiness, which returns an error for the "error" state we just set. that wait error is
				// expected here and means the update actually succeeded.
				if strings.Contains(errors.RootCause(err).Error(), "in error state") {
					s.Logger.DebugWithCtx(ctx, "Stale function state set to error",
						"functionName", functionConfig.Meta.Name)
					return
				}

				// non-fatal: keep sweeping the remaining functions
				s.Logger.WarnWithCtx(ctx, "Failed to set stale function state to error",
					"functionName", functionConfig.Meta.Name,
					"err", err.Error())
				return
			}

			s.Logger.DebugWithCtx(ctx, "Marked stale function as error",
				"functionName", functionConfig.Meta.Name)
		})
	}

	wg.Wait()
	s.Logger.DebugWithCtx(ctx, "Finished marking stale functions as error")
}

func (s *Server) resolveDockerCredentialsRegistryURL(credentials dockercreds.Credentials) string {
	registryURL := credentials.URL

	// TODO: This auto-expansion does not support with kaniko today, must provide full URL. Remove this?
	// if the user specified the docker hub, we can't use this as-is. add the user name to the URL
	// to generate a valid URL
	if common.MatchStringPatterns([]string{
		`^.*\.docker\.com.*$`,
		`^.*\.docker\.io.*$`,
	}, registryURL) {
		registryURL = common.StripSuffixes(registryURL, []string{

			// when using docker.io as login address, the resolved address in the docker credentials file
			// might contain the registry version, strip it if so
			"/v1",
			"/v1/",
		})

		// if no slash after the URL default to the provided username
		if !common.MatchStringPatterns([]string{
			`\.docker\.com\/`,
			`\.docker\.io\/`,
		}, registryURL) {
			registryURL = fmt.Sprintf("%s/%s", registryURL, credentials.Username)
		}
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
