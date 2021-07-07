package app

import (
	"github.com/nuclio/nuclio/pkg/platform"
	"github.com/nuclio/nuclio/pkg/platformconfig"

	"github.com/nuclio/logger"
)

type CreateDashboardServerOptions struct {
	logger                logger.Logger
	platformConfiguration *platformconfig.Config
	platformInstance      platform.Platform

	// arguments
	listenAddress                    string
	dockerKeyDir                     string
	defaultRegistryURL               string
	defaultRunRegistryURL            string
	platformType                     string
	noPullBaseImages                 bool
	defaultCredRefreshIntervalString string
	externalIPAddresses              string
	defaultNamespace                 string
	offline                          bool
	templatesGitRepository           string
	templatesGitRef                  string
	templatesArchiveAddress          string
	templatesGitUsername             string
	templatesGitPassword             string
	templatesGithubAccessToken       string
	templatesGitCaCertContents       string
	imageNamePrefixTemplate          string
	platformAuthorizationMode        string
	dependantImageRegistryURL        string
}
