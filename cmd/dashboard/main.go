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

package main

import (
	"flag"
	"os"

	"github.com/nuclio/nuclio/cmd/dashboard/app"
	"github.com/nuclio/nuclio/pkg/common"
	_ "github.com/nuclio/nuclio/pkg/dashboard/resource"

	"github.com/nuclio/errors"
)

func getNamespace(namespaceArgument string) string {

	// if the namespace was passed in the arguments, use that
	if namespaceArgument != "" {
		return namespaceArgument
	}

	// if the namespace exists in env, use that
	if namespaceEnv := os.Getenv("NUCLIO_DASHBOARD_NAMESPACE"); namespaceEnv != "" {
		return namespaceEnv
	}

	// if nothing was passed, assume "this" namespace
	return "@nuclio.selfNamespace"
}

func main() {
	defaultNoPullBaseImages := os.Getenv("NUCLIO_DASHBOARD_NO_PULL_BASE_IMAGES") == "true"
	defaultOffline := os.Getenv("NUCLIO_DASHBOARD_OFFLINE") == "true"

	externalIPAddressesDefault := os.Getenv("NUCLIO_DASHBOARD_EXTERNAL_IP_ADDRESSES")

	defaultPlatformAuthorizationMode := os.Getenv("NUCLIO_DASHBOARD_PLATFORM_AUTHORIZATION_MODE")
	if defaultPlatformAuthorizationMode == "" {
		defaultPlatformAuthorizationMode = "service-account"
	}

	// git templating env vars
	templatesGitRepository := flag.String("templates-git-repository", common.GetEnvOrDefaultString("NUCLIO_TEMPLATES_GIT_REPOSITORY", "https://github.com/nuclio/nuclio-templates.git"), "Git templates repo's name")
	templatesGitBranch := flag.String("templates-git-ref", common.GetEnvOrDefaultString("NUCLIO_TEMPLATES_GIT_REF", "refs/heads/master"), "Git templates repo's branch name")
	templatesGitUsername := flag.String("templates-git-username", common.GetEnvOrDefaultString("NUCLIO_TEMPLATES_GIT_USERNAME", ""), "Git repo's username")
	templatesGitPassword := flag.String("templates-git-password", common.GetEnvOrDefaultString("NUCLIO_TEMPLATES_GIT_PASSWORD", ""), "Git repo's user password")
	templatesGithubAccessToken := flag.String("templates-github-access-token", common.GetEnvOrDefaultString("NUCLIO_TEMPLATES_GITHUB_ACCESS_TOKEN", ""), "Github templates repo's access token")
	templatesArchiveAddress := flag.String("templates-archive-address", common.GetEnvOrDefaultString("NUCLIO_TEMPLATES_ARCHIVE_ADDRESS", ""), "Function Templates zip file address")

	listenAddress := flag.String("listen-addr", ":8070", "IP/port on which the playground listens")
	dockerKeyDir := flag.String("docker-key-dir", "", "Directory to look for docker keys for secure registries")
	platformType := flag.String("platform", "auto", "One of kube/local/auto")
	defaultRegistryURL := flag.String("registry", os.Getenv("NUCLIO_DASHBOARD_REGISTRY_URL"), "Default registry URL")
	defaultRunRegistryURL := flag.String("run-registry", os.Getenv("NUCLIO_DASHBOARD_RUN_REGISTRY_URL"), "Default run registry URL")
	noPullBaseImages := flag.Bool("no-pull", defaultNoPullBaseImages, "Default run registry URL")
	credsRefreshInterval := flag.String("creds-refresh-interval", os.Getenv("NUCLIO_DASHBOARD_CREDS_REFRESH_INTERVAL"), "Default credential refresh interval, or 'none' (12h by default)")
	externalIPAddresses := flag.String("external-ip-addresses", externalIPAddressesDefault, "Comma delimited list of external IP addresses")
	namespace := flag.String("namespace", "", "Namespace in which all actions apply to, if not passed in request")
	offline := flag.Bool("offline", defaultOffline, "If true, assumes no internet connectivity")
	platformConfigurationPath := flag.String("platform-config", "/etc/nuclio/config/platform/platform.yaml", "Path of platform configuration file")
	defaultHTTPIngressHostTemplate := flag.String("default-http-ingress-host-template", os.Getenv("NUCLIO_DASHBOARD_HTTP_INGRESS_HOST_TEMPLATE"), "Go template for the default http ingress host")
	imageNamePrefixTemplate := flag.String("image-name-prefix-template", os.Getenv("NUCLIO_DASHBOARD_IMAGE_NAME_PREFIX_TEMPLATE"), "Go template for the image names prefix")
	platformAuthorizationMode := flag.String("platform-authorization-mode", defaultPlatformAuthorizationMode, "One of service-account (default) / authorization-header-oidc")
	dependantImageRegistryURL := flag.String("dependant-image-registry", os.Getenv("NUCLIO_DASHBOARD_DEPENDANT_IMAGE_REGISTRY_URL"), "If passed, replaces base/on-build registry URLs with this value")

	// get the namespace from args -> env -> default
	*namespace = getNamespace(*namespace)

	flag.Parse()

	if err := app.Run(*listenAddress,
		*dockerKeyDir,
		*defaultRegistryURL,
		*defaultRunRegistryURL,
		*platformType,
		*noPullBaseImages,
		*credsRefreshInterval,
		*externalIPAddresses,
		*namespace,
		*offline,
		*platformConfigurationPath,
		*templatesGitRepository,
		*templatesGitBranch,
		*templatesArchiveAddress,
		*templatesGitUsername,
		*templatesGitPassword,
		*templatesGithubAccessToken,
		*defaultHTTPIngressHostTemplate,
		*imageNamePrefixTemplate,
		*platformAuthorizationMode,
		*dependantImageRegistryURL); err != nil {

		errors.PrintErrorStack(os.Stderr, err, 5)

		os.Exit(1)
	}

	os.Exit(0)
}
