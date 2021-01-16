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
	"strings"
	"time"

	commonhealthcheck "github.com/nuclio/nuclio/pkg/common/healthcheck"
	"github.com/nuclio/nuclio/pkg/common/statusprovider"
	"github.com/nuclio/nuclio/pkg/dashboard"
	"github.com/nuclio/nuclio/pkg/dashboard/functiontemplates"
	"github.com/nuclio/nuclio/pkg/dashboard/healthcheck"
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
	defaultHTTPIngressHostTemplate string,
	imageNamePrefixTemplate string,
	platformAuthorizationMode string,
	dependantImageRegistryURL string) error {

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
		status: statusprovider.Initializing,
	}

	dashboardInstance.healthCheckServer, err = createAndStartHealthCheckServer(platformConfiguration,
		rootLogger,
		dashboardInstance)
	if err != nil {
		return errors.Wrap(err, "Failed to create and start health check server")
	}

	dashboardInstance.server, err = newDashboardServer(&CreateDashboardServerOptions{
		logger:                dashboardInstance.logger,
		platformConfiguration: platformConfiguration,

		ListenAddress:                    listenAddress,
		DockerKeyDir:                     dockerKeyDir,
		DefaultRegistryURL:               defaultRegistryURL,
		DefaultRunRegistryURL:            defaultRunRegistryURL,
		PlatformType:                     platformType,
		NoPullBaseImages:                 noPullBaseImages,
		DefaultCredRefreshIntervalString: defaultCredRefreshIntervalString,
		ExternalIPAddresses:              externalIPAddresses,
		DefaultNamespace:                 defaultNamespace,
		Offline:                          offline,
		TemplatesGitRepository:           templatesGitRepository,
		TemplatesGitRef:                  templatesGitRef,
		TemplatesArchiveAddress:          templatesArchiveAddress,
		TemplatesGitUsername:             templatesGitUsername,
		TemplatesGitPassword:             templatesGitPassword,
		TemplatesGithubAccessToken:       templatesGithubAccessToken,
		DefaultHTTPIngressHostTemplate:   defaultHTTPIngressHostTemplate,
		ImageNamePrefixTemplate:          imageNamePrefixTemplate,
		PlatformAuthorizationMode:        platformAuthorizationMode,
		DependantImageRegistryURL:        dependantImageRegistryURL,
	})
	if err != nil {
		return errors.Wrap(err, "Failed to create new dashboard")
	}

	// TODO: receive from function args
	go dashboardInstance.MonitorDockerConnectivity(10*time.Second, 5)

	if err := dashboardInstance.server.Start(); err != nil {
		return errors.Wrap(err, "Failed to start server")
	}

	dashboardInstance.status = statusprovider.Ready
	select {}
}

func newDashboardServer(createDashboardServerOptions *CreateDashboardServerOptions) (restful.Server, error) {
	rootLogger := createDashboardServerOptions.logger
	var functionGitTemplateFetcher *functiontemplates.GitFunctionTemplateFetcher
	var functionZipTemplateFetcher *functiontemplates.ZipFunctionTemplateFetcher

	// create a platform
	platformInstance, err := factory.CreatePlatform(rootLogger,
		createDashboardServerOptions.PlatformType,
		createDashboardServerOptions.platformConfiguration,
		createDashboardServerOptions.DefaultNamespace)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to create platform")
	}

	// create git fetcher
	if createDashboardServerOptions.TemplatesGitRepository != "" &&
		createDashboardServerOptions.TemplatesGitRef != "" {
		rootLogger.DebugWith("Fetching function templates from git repository",
			"templatesGitRepository", createDashboardServerOptions.TemplatesGitRepository,
			"templatesGitRef", createDashboardServerOptions.TemplatesGitRef)

		// attach credentials if given
		templatesGitRepository := attachCredentialsToGitRepository(createDashboardServerOptions.logger,
			createDashboardServerOptions.TemplatesGitRepository,
			createDashboardServerOptions.TemplatesGitUsername,
			createDashboardServerOptions.TemplatesGitPassword,
			createDashboardServerOptions.TemplatesGithubAccessToken)

		functionGitTemplateFetcher, err = functiontemplates.NewGitFunctionTemplateFetcher(rootLogger,
			templatesGitRepository,
			createDashboardServerOptions.TemplatesGitRef)
		if err != nil {
			return nil, errors.Wrap(err, "Failed to create git fetcher")
		}
	} else {
		rootLogger.DebugWith("Missing git fetcher configuration, templates from git won't be fetched",
			"gitTemplateRepository", createDashboardServerOptions.TemplatesGitRepository,
			"templatesGitRef", createDashboardServerOptions.TemplatesGitRef)
	}

	// create zip fetcher
	if createDashboardServerOptions.TemplatesArchiveAddress != "" {
		functionZipTemplateFetcher, err = functiontemplates.NewZipFunctionTemplateFetcher(rootLogger,
			createDashboardServerOptions.TemplatesArchiveAddress)
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
	if createDashboardServerOptions.ExternalIPAddresses == "" {
		splitExternalIPAddresses, err = platformInstance.GetDefaultInvokeIPAddresses()
		if err != nil {
			return nil, errors.Wrap(err, "Failed to get default invoke ip addresses")
		}
	} else {

		// "10.0.0.1,10.0.0.2" -> ["10.0.0.1", "10.0.0.2"]
		splitExternalIPAddresses = strings.Split(createDashboardServerOptions.ExternalIPAddresses, ",")
	}

	if err := platformInstance.SetExternalIPAddresses(splitExternalIPAddresses); err != nil {
		return nil, errors.Wrap(err, "Failed to set external ip addresses")
	}

	if createDashboardServerOptions.DefaultHTTPIngressHostTemplate != "" {
		platformInstance.SetDefaultHTTPIngressHostTemplate(createDashboardServerOptions.DefaultHTTPIngressHostTemplate)
	}

	if createDashboardServerOptions.ImageNamePrefixTemplate != "" {
		platformInstance.SetImageNamePrefixTemplate(createDashboardServerOptions.ImageNamePrefixTemplate)
	}

	createDashboardServerOptions.logger.InfoWith("Starting dashboard",
		"name", platformInstance.GetName(),
		"noPull", createDashboardServerOptions.NoPullBaseImages,
		"offline", createDashboardServerOptions.Offline,
		"defaultCredRefreshInterval", createDashboardServerOptions.DefaultCredRefreshIntervalString,
		"defaultNamespace", createDashboardServerOptions.DefaultNamespace,
		"version", version.Get(),
		"platformConfiguration", createDashboardServerOptions.platformConfiguration,
		"containerBuilderKind", platformInstance.GetContainerBuilderKind())

	// create a web server configuration
	trueValue := true
	webServerConfiguration := &platformconfig.WebServer{
		Enabled:       &trueValue,
		ListenAddress: createDashboardServerOptions.ListenAddress,
	}

	dashboardServer, err := dashboard.NewServer(rootLogger,
		platformInstance.GetContainerBuilderKind(),
		createDashboardServerOptions.DockerKeyDir,
		createDashboardServerOptions.DefaultRegistryURL,
		createDashboardServerOptions.DefaultRunRegistryURL,
		platformInstance,
		createDashboardServerOptions.NoPullBaseImages,
		webServerConfiguration,
		getDefaultCredRefreshInterval(rootLogger, createDashboardServerOptions.DefaultCredRefreshIntervalString),
		splitExternalIPAddresses,
		platformInstance.ResolveDefaultNamespace(createDashboardServerOptions.DefaultNamespace),
		createDashboardServerOptions.Offline,
		functionTemplatesRepository,
		createDashboardServerOptions.platformConfiguration,
		createDashboardServerOptions.DefaultHTTPIngressHostTemplate,
		createDashboardServerOptions.ImageNamePrefixTemplate,
		createDashboardServerOptions.PlatformAuthorizationMode,
		createDashboardServerOptions.DependantImageRegistryURL)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to create server")
	}

	return dashboardServer, nil
}

func createAndStartHealthCheckServer(platformConfiguration *platformconfig.Config,
	loggerInstance logger.Logger,
	dashboardInstance *Dashboard) (commonhealthcheck.Server, error) {

	// if enabled not passed, default to true
	if platformConfiguration.HealthCheck.Enabled == nil {
		trueValue := true
		platformConfiguration.HealthCheck.Enabled = &trueValue
	}

	if platformConfiguration.HealthCheck.ListenAddress == "" {
		platformConfiguration.HealthCheck.ListenAddress = ":8082"
	}

	// create the server
	server, err := healthcheck.NewDashboardServer(loggerInstance, dashboardInstance, &platformConfiguration.HealthCheck)
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
