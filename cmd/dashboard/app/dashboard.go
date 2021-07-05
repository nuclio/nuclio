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

package app

import (
	"context"
	"strconv"
	"strings"
	"time"

	commonhealthcheck "github.com/nuclio/nuclio/pkg/common/healthcheck"
	"github.com/nuclio/nuclio/pkg/common/status"
	"github.com/nuclio/nuclio/pkg/dashboard"
	"github.com/nuclio/nuclio/pkg/dashboard/functiontemplates"
	"github.com/nuclio/nuclio/pkg/dashboard/healthcheck"
	"github.com/nuclio/nuclio/pkg/dockerclient"
	"github.com/nuclio/nuclio/pkg/loggersink"
	"github.com/nuclio/nuclio/pkg/platform/factory"
	"github.com/nuclio/nuclio/pkg/platformconfig"
	"github.com/nuclio/nuclio/pkg/restful"
	// load all sinks
	_ "github.com/nuclio/nuclio/pkg/sinks"

	"github.com/nuclio/errors"
	"github.com/nuclio/logger"
	"github.com/v3io/version-go"
)

type Dashboard struct {
	server            restful.Server
	healthCheckServer commonhealthcheck.Server
	logger            logger.Logger

	status status.Status
}

func (d *Dashboard) GetStatus() status.Status {
	return d.status
}

func (d *Dashboard) SetStatus(status status.Status) {
	if d.status != status {
		d.logger.InfoWith("Updating server healthiness",
			"currentStatus", d.status,
			"desiredStatus", status)
	}
	d.status = status
}

func (d *Dashboard) MonitorDockerConnectivity(ctx context.Context,
	interval time.Duration,
	maxConsecutiveErrors int,
	dockerClient dockerclient.Client) {

	d.logger.DebugWith("Monitoring docker connectivity",
		"interval", interval,
		"maxConsecutiveErrors", maxConsecutiveErrors)

	consecutiveErrors := 0
	dockerConnectivityTicker := time.NewTicker(interval)
	for {
		select {
		case <-ctx.Done():
			dockerConnectivityTicker.Stop()
			d.logger.DebugWith("Stopping docker connectivity monitor")
			return
		case <-dockerConnectivityTicker.C:
			if d.GetStatus().OneOf(status.Error) {

				// do not monitor while status is error
				// let the kubelet / dockerd / user restart the process
				continue
			}

			// get version quietly, avoid execution spamming stdout
			if _, err := dockerClient.GetVersion(true); err == nil {

				// reset counter
				consecutiveErrors = 0
				continue
			}
			consecutiveErrors++
			if consecutiveErrors == maxConsecutiveErrors {
				d.SetStatus(status.Error)
				d.logger.Error("Failed to resolve docker version, connection might be unhealthy.")
			}
		}
	}
}

func Run(listenAddress string,
	dockerKeyDir string,
	defaultRegistryURL string,
	defaultRunRegistryURL string,
	platformType string,
	noPullBaseImages bool,
	defaultCredRefreshIntervalString string,
	externalIPAddresses string,
	defaultNamespace string,
	offline bool,
	platformConfigurationPath string,
	templatesGitRepository string,
	templatesGitRef string,
	templatesArchiveAddress string,
	templatesGitUsername string,
	templatesGitPassword string,
	templatesGithubAccessToken string,
	templatesGitCaCertContents string,
	imageNamePrefixTemplate string,
	platformAuthorizationMode string,
	dependantImageRegistryURL string,
	monitorDockerDeamon bool,
	monitorDockerDeamonIntervalStr string,
	monitorDockerDeamonMaxConsecutiveErrorsStr string) error {

	// get platform configuration
	platformConfiguration, err := platformconfig.NewPlatformConfig(platformConfigurationPath)
	if err != nil {
		return errors.Wrap(err, "Failed to get platform configuration")
	}

	// create a root logger
	rootLogger, err := loggersink.CreateSystemLogger("dashboard", platformConfiguration)
	if err != nil {
		return errors.Wrap(err, "Failed to create logger")
	}

	dashboardInstance := &Dashboard{
		logger: rootLogger,
		status: status.Initializing,
	}

	dashboardInstance.healthCheckServer, err = createAndStartHealthCheckServer(platformConfiguration,
		rootLogger,
		dashboardInstance)
	if err != nil {
		return errors.Wrap(err, "Failed to create and start health check server")
	}

	// create a platform
	platformInstance, err := factory.CreatePlatform(rootLogger,
		platformType,
		platformConfiguration,
		defaultNamespace)
	if err != nil {
		return errors.Wrap(err, "Failed to create platform")
	}

	dashboardInstance.server, err = newDashboardServer(&CreateDashboardServerOptions{
		logger:                dashboardInstance.logger,
		platformConfiguration: platformConfiguration,
		platformInstance:      platformInstance,

		// arguments
		listenAddress:                    listenAddress,
		dockerKeyDir:                     dockerKeyDir,
		defaultRegistryURL:               defaultRegistryURL,
		defaultRunRegistryURL:            defaultRunRegistryURL,
		platformType:                     platformType,
		noPullBaseImages:                 noPullBaseImages,
		defaultCredRefreshIntervalString: defaultCredRefreshIntervalString,
		externalIPAddresses:              externalIPAddresses,
		defaultNamespace:                 defaultNamespace,
		offline:                          offline,
		templatesGitRepository:           templatesGitRepository,
		templatesGitRef:                  templatesGitRef,
		templatesArchiveAddress:          templatesArchiveAddress,
		templatesGitUsername:             templatesGitUsername,
		templatesGitPassword:             templatesGitPassword,
		templatesGithubAccessToken:       templatesGithubAccessToken,
		templatesGitCaCertContents:       templatesGitCaCertContents,
		imageNamePrefixTemplate:          imageNamePrefixTemplate,
		platformAuthorizationMode:        platformAuthorizationMode,
		dependantImageRegistryURL:        dependantImageRegistryURL,
	})
	if err != nil {
		return errors.Wrap(err, "Failed to create new dashboard")
	}

	// monitor docker connectivity to quickly populate any issue while connecting to docker daemon
	if monitorDockerDeamon && platformInstance.GetContainerBuilderKind() == "docker" {

		// parse docker deamon monitor max consecutive errors
		monitorDockerDeamonMaxConsecutiveErrors, err := strconv.Atoi(monitorDockerDeamonMaxConsecutiveErrorsStr)
		if err != nil {
			return errors.Wrapf(err, "Failed to parse string '%s' to integer", monitorDockerDeamonMaxConsecutiveErrorsStr)
		}

		// parse docker deamon monitor interval
		monitorDockerDeamonInterval, err := time.ParseDuration(monitorDockerDeamonIntervalStr)
		if err != nil {
			return errors.Wrapf(err, "Failed to parse string '%s' to duration", monitorDockerDeamonIntervalStr)
		}

		// create docker client
		dockerClient, err := dockerclient.NewShellClient(rootLogger, nil)
		if err != nil {
			return errors.Wrap(err, "Failed to create docker shell client")
		}

		ctx, cancel := context.WithCancel(context.Background())
		go dashboardInstance.MonitorDockerConnectivity(ctx,
			monitorDockerDeamonInterval,
			monitorDockerDeamonMaxConsecutiveErrors,
			dockerClient)
		defer cancel()
	}

	if err := dashboardInstance.server.Start(); err != nil {
		return errors.Wrap(err, "Failed to start server")
	}

	dashboardInstance.status = status.Ready
	select {}
}

func newDashboardServer(createDashboardServerOptions *CreateDashboardServerOptions) (restful.Server, error) {
	rootLogger := createDashboardServerOptions.logger
	var err error
	var functionGitTemplateFetcher *functiontemplates.GitFunctionTemplateFetcher
	var functionZipTemplateFetcher *functiontemplates.ZipFunctionTemplateFetcher

	// shorter
	platformInstance := createDashboardServerOptions.platformInstance

	// create git fetcher
	if createDashboardServerOptions.templatesGitRepository != "" &&
		createDashboardServerOptions.templatesGitRef != "" {
		rootLogger.DebugWith("Fetching function templates from git repository",
			"templatesGitRepository", createDashboardServerOptions.templatesGitRepository,
			"templatesGitRef", createDashboardServerOptions.templatesGitRef)

		// attach credentials if given
		templatesGitRepository := attachCredentialsToGitRepository(createDashboardServerOptions.logger,
			createDashboardServerOptions.templatesGitRepository,
			createDashboardServerOptions.templatesGitUsername,
			createDashboardServerOptions.templatesGitPassword,
			createDashboardServerOptions.templatesGithubAccessToken)

		functionGitTemplateFetcher, err = functiontemplates.NewGitFunctionTemplateFetcher(rootLogger,
			templatesGitRepository,
			createDashboardServerOptions.templatesGitRef,
			createDashboardServerOptions.templatesGitCaCertContents)
		if err != nil {
			return nil, errors.Wrap(err, "Failed to create git fetcher")
		}
	} else {
		rootLogger.DebugWith("Missing git fetcher configuration, templates from git won't be fetched",
			"gitTemplateRepository", createDashboardServerOptions.templatesGitRepository,
			"templatesGitRef", createDashboardServerOptions.templatesGitRef)
	}

	// create zip fetcher
	if createDashboardServerOptions.templatesArchiveAddress != "" {
		functionZipTemplateFetcher, err = functiontemplates.NewZipFunctionTemplateFetcher(rootLogger,
			createDashboardServerOptions.templatesArchiveAddress)
		if err != nil {
			return nil, errors.Wrap(err, "Failed to create zip template fetcher")
		}
	}

	// create pre-generated templates fetcher
	functionTemplatesGeneratedFetcher, err := functiontemplates.NewGeneratedFunctionTemplateFetcher(rootLogger)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to create pre-generated fetcher")
	}

	// make repository for fetchers
	functionTemplateFetchers := []functiontemplates.FunctionTemplateFetcher{functionTemplatesGeneratedFetcher}

	if functionGitTemplateFetcher != nil {
		functionTemplateFetchers = append(functionTemplateFetchers, functionGitTemplateFetcher)
	}

	if functionZipTemplateFetcher != nil {
		functionTemplateFetchers = append(functionTemplateFetchers, functionZipTemplateFetcher)
	}

	functionTemplatesRepository, err := functiontemplates.NewRepository(rootLogger, functionTemplateFetchers)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to create repository out of given fetchers")
	}

	// set external ip addresses based if user passed overriding values or not
	var splitExternalIPAddresses []string
	if createDashboardServerOptions.externalIPAddresses == "" {
		splitExternalIPAddresses, err = platformInstance.GetDefaultInvokeIPAddresses()
		if err != nil {
			return nil, errors.Wrap(err, "Failed to get default invoke ip addresses")
		}
	} else {

		// "10.0.0.1,10.0.0.2" -> ["10.0.0.1", "10.0.0.2"]
		splitExternalIPAddresses = strings.Split(createDashboardServerOptions.externalIPAddresses, ",")
	}

	if err := platformInstance.SetExternalIPAddresses(splitExternalIPAddresses); err != nil {
		return nil, errors.Wrap(err, "Failed to set external ip addresses")
	}

	if createDashboardServerOptions.imageNamePrefixTemplate != "" {
		platformInstance.SetImageNamePrefixTemplate(createDashboardServerOptions.imageNamePrefixTemplate)
	}

	createDashboardServerOptions.logger.InfoWith("Starting dashboard",
		"name", platformInstance.GetName(),
		"noPull", createDashboardServerOptions.noPullBaseImages,
		"offline", createDashboardServerOptions.offline,
		"defaultCredRefreshInterval", createDashboardServerOptions.defaultCredRefreshIntervalString,
		"defaultNamespace", createDashboardServerOptions.defaultNamespace,
		"version", version.Get(),
		"platformConfiguration", createDashboardServerOptions.platformConfiguration,
		"containerBuilderKind", platformInstance.GetContainerBuilderKind())

	// create a web server configuration
	trueValue := true
	webServerConfiguration := &platformconfig.WebServer{
		Enabled:       &trueValue,
		ListenAddress: createDashboardServerOptions.listenAddress,
	}

	dashboardServer, err := dashboard.NewServer(rootLogger,
		platformInstance.GetContainerBuilderKind(),
		createDashboardServerOptions.dockerKeyDir,
		createDashboardServerOptions.defaultRegistryURL,
		createDashboardServerOptions.defaultRunRegistryURL,
		platformInstance,
		createDashboardServerOptions.noPullBaseImages,
		webServerConfiguration,
		getDefaultCredRefreshInterval(rootLogger, createDashboardServerOptions.defaultCredRefreshIntervalString),
		splitExternalIPAddresses,
		platformInstance.ResolveDefaultNamespace(createDashboardServerOptions.defaultNamespace),
		createDashboardServerOptions.offline,
		functionTemplatesRepository,
		createDashboardServerOptions.platformConfiguration,
		createDashboardServerOptions.imageNamePrefixTemplate,
		createDashboardServerOptions.platformAuthorizationMode,
		createDashboardServerOptions.dependantImageRegistryURL)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to create server")
	}

	return dashboardServer, nil
}

func createAndStartHealthCheckServer(platformConfiguration *platformconfig.Config,
	loggerInstance logger.Logger,
	statusProvider status.Provider) (commonhealthcheck.Server, error) {

	// if enabled not passed, default to true
	if platformConfiguration.HealthCheck.Enabled == nil {
		trueValue := true
		platformConfiguration.HealthCheck.Enabled = &trueValue
	}

	if platformConfiguration.HealthCheck.ListenAddress == "" {
		platformConfiguration.HealthCheck.ListenAddress = ":8082"
	}

	// create the server
	server, err := healthcheck.NewDashboardServer(loggerInstance, statusProvider, &platformConfiguration.HealthCheck)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to create health check server")
	}

	// start the web interface
	if err := server.Start(); err != nil {
		return nil, errors.Wrap(err, "Failed to start health check server")
	}

	return server, nil
}

func getDefaultCredRefreshInterval(logger logger.Logger, defaultCredRefreshIntervalString string) *time.Duration {
	var defaultCredRefreshInterval time.Duration
	defaultInterval := 12 * time.Hour

	// if set to "none" - no refresh interval
	if defaultCredRefreshIntervalString == "none" {
		return nil
	}

	// if unspecified, default to 12 hours
	if defaultCredRefreshIntervalString == "" {
		return &defaultInterval
	}

	// try to parse the refresh interval - if failed
	defaultCredRefreshInterval, err := time.ParseDuration(defaultCredRefreshIntervalString)
	if err != nil {
		logger.WarnWith("Failed to parse default credential refresh interval, defaulting",
			"given", defaultCredRefreshIntervalString,
			"default", defaultInterval)

		return &defaultInterval
	}

	return &defaultCredRefreshInterval
}

// create new repo URL with the credentials inside of it (when credentials are passed)
// example: https://github.com/owner/repo.git -> https://<USERNAME>:<PASSWORD>@github.com/owner/repo.git
func attachCredentialsToGitRepository(logger logger.Logger, repo, username, password, accessToken string) string {
	if accessToken != "" {
		username = accessToken
		password = "x-oauth-basic"
	} else if username == "" || password == "" {
		return repo
	}

	splitRepo := strings.Split(repo, "//")
	if len(splitRepo) != 2 {
		logger.WarnWith("Unknown git repository structure. Skipping credentials attachment", "repo", repo)
		return repo
	}
	return strings.Join([]string{splitRepo[0], "//", username, ":", password, "@", splitRepo[1]}, "")
}
