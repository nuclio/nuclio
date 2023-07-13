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

package app

import (
	"github.com/nuclio/nuclio/pkg/auth"
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

	// auth options
	authConfig *auth.Config
}
