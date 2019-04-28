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

	"github.com/nuclio/nuclio/pkg/dashboard"
	"github.com/nuclio/nuclio/pkg/dashboard/functiontemplates"
	"github.com/nuclio/nuclio/pkg/errors"
	"github.com/nuclio/nuclio/pkg/loggersink"
	"github.com/nuclio/nuclio/pkg/platform/factory"
	"github.com/nuclio/nuclio/pkg/platformconfig"
	// load all sinks
	_ "github.com/nuclio/nuclio/pkg/sinks"
	"github.com/nuclio/nuclio/pkg/version"

	"github.com/nuclio/logger"
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
	templatesGithubAccessToken string,) error {
	var functionGitTemplateFetcher *functiontemplates.GitFunctionTemplateFetcher
	var functionZipTemplateFetcher *functiontemplates.ZipFunctionTemplateFetcher

	// read platform configuration
	platformConfiguration, err := readPlatformConfiguration(platformConfigurationPath)
	if err != nil {
		return errors.Wrap(err, "Failed to read platform configuration")
	}

	// create a root logger
	rootLogger, err := loggersink.CreateSystemLogger("dashboard", platformConfiguration)
	if err != nil {
		return errors.Wrap(err, "Failed to create logger")
	}

	// create a platform
	platformInstance, err := factory.CreatePlatform(rootLogger, platformType, nil)
	if err != nil {
		return errors.Wrap(err, "Failed to create platform")
	}

	// create git fetcher
	if templatesGitRepository != "" && templatesGitRef != "" {
		rootLogger.DebugWith("Fetching function templates from git repository",
			"templatesGitRepository", templatesGitRepository,
			"templatesGitRef", templatesGitRef)

		// attach credentials if given
		templatesGitRepository = attachCredentialsToGitRepository(rootLogger,
			templatesGitRepository,
			templatesGitUsername,
			templatesGitPassword,
			templatesGithubAccessToken)

		functionGitTemplateFetcher, err = functiontemplates.NewGitFunctionTemplateFetcher(rootLogger,
			templatesGitRepository,
			templatesGitRef)
		if err != nil {
			return errors.Wrap(err, "Failed to create git fetcher")
		}
	} else {
		rootLogger.DebugWith("Missing git fetcher configuration, templates from git won't be fetched",
			"gitTemplateRepository", templatesGitRepository,
			"templatesGitRef", templatesGitRef)
	}

	// create zip fetcher
	if templatesArchiveAddress != "" {
		functionZipTemplateFetcher, err = functiontemplates.NewZipFunctionTemplateFetcher(rootLogger,
			templatesArchiveAddress)
		if err != nil {
			return errors.Wrap(err, "Failed to create zip template fetcher")
		}
	}

	// create pre-generated templates fetcher
	functionTemplatesGeneratedFetcher, err := functiontemplates.NewGeneratedFunctionTemplateFetcher(rootLogger)
	if err != nil {
		return errors.Wrap(err, "Failed to create pre-generated fetcher")
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
		return errors.Wrap(err, "Failed to create repository out of given fetchers")
	}

	// set external ip addresses based if user passed overriding values or not
	var splitExternalIPAddresses []string
	if externalIPAddresses == "" {
		splitExternalIPAddresses, err = platformInstance.GetDefaultInvokeIPAddresses()
		if err != nil {
			return errors.Wrap(err, "Failed to get default invoke ip addresses")
		}
	} else {

		// "10.0.0.1,10.0.0.2" -> ["10.0.0.1", "10.0.0.2"]
		splitExternalIPAddresses = strings.Split(externalIPAddresses, ",")
	}

	err = platformInstance.SetExternalIPAddresses(splitExternalIPAddresses)
	if err != nil {
		return errors.Wrap(err, "Failed to set external ip addresses")
	}

	rootLogger.InfoWith("Starting",
		"name", platformInstance.GetName(),
		"noPull", noPullBaseImages,
		"offline", offline,
		"defaultCredRefreshInterval", defaultCredRefreshIntervalString,
		"defaultNamespace", defaultNamespace,
		"platformConfiguration", platformConfiguration)

	// see if the platform has anything to say about the namespace
	defaultNamespace = platformInstance.ResolveDefaultNamespace(defaultNamespace)

	version.Log(rootLogger)

	trueValue := true

	// create a web server configuration
	webServerConfiguration := &platformconfig.WebServer{
		Enabled:       &trueValue,
		ListenAddress: listenAddress,
	}

	server, err := dashboard.NewServer(rootLogger,
		dockerKeyDir,
		defaultRegistryURL,
		defaultRunRegistryURL,
		platformInstance,
		noPullBaseImages,
		webServerConfiguration,
		getDefaultCredRefreshInterval(rootLogger, defaultCredRefreshIntervalString),
		splitExternalIPAddresses,
		defaultNamespace,
		offline,
		functionTemplatesRepository,
		platformConfiguration)
	if err != nil {
		return errors.Wrap(err, "Failed to create server")
	}

	err = server.Start()
	if err != nil {
		return errors.Wrap(err, "Failed to start server")
	}

	select {}
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

func readPlatformConfiguration(configurationPath string) (*platformconfig.Config, error) {
	platformConfigurationReader, err := platformconfig.NewReader()
	if err != nil {
		return nil, errors.Wrap(err, "Failed to create platform configuration reader")
	}

	return platformConfigurationReader.ReadFileOrDefault(configurationPath)
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
	return strings.Join([]string{splitRepo[0],"//", username, ":", password, "@", splitRepo[1]},"")
}
