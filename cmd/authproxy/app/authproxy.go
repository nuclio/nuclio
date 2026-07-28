/*
Copyright 2026 The Nuclio Authors.

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
	"github.com/nuclio/nuclio/pkg/auth/authproxy"
	"github.com/nuclio/nuclio/pkg/loggersink"
	"github.com/nuclio/nuclio/pkg/platformconfig"

	"github.com/nuclio/errors"
	"github.com/nuclio/logger"

	// register logger sinks (stdout, appinsights) into the loggersink registry via init()
	_ "github.com/nuclio/nuclio/pkg/sinks"
)

// Run creates and starts the auth-proxy server for the given configuration.
func Run(config *Config) error {
	if err := validateConfiguration(config); err != nil {
		return errors.Wrap(err, "Invalid auth-proxy configuration")
	}

	platformConfiguration, err := platformconfig.NewPlatformConfig(config.PlatformConfigPath)
	if err != nil {
		return errors.Wrap(err, "Failed to get platform configuration")
	}

	rootLogger, err := loggersink.CreateSystemLogger("auth-proxy", platformConfiguration)
	if err != nil {
		return errors.Wrap(err, "Failed to create logger")
	}

	authenticator, err := newAuthenticator(rootLogger, config)
	if err != nil {
		return errors.Wrap(err, "Failed to create authenticator")
	}

	handler, err := newHandler(rootLogger,
		config.Mode,
		config.UpstreamURL,
		authenticator)
	if err != nil {
		return errors.Wrap(err, "Failed to create handler")
	}

	listenAddress, err := resolveListenAddress(config.Mode, config.ListenPort)
	if err != nil {
		return errors.Wrap(err, "Failed to resolve listen address")
	}

	server := newServer(rootLogger, listenAddress, handler)
	if err := server.start(); err != nil {
		return errors.Wrap(err, "Failed to start auth-proxy server")
	}
	select {}
}

// newAuthenticator creates kube clients when needed and delegates to the pkg-level factory.
func newAuthenticator(rootLogger logger.Logger, config *Config) (authproxy.Authenticator, error) {
	return authproxy.NewAuthenticator(rootLogger,
		config.Mode,
		config.AuthURL,
		config.SigninURL,
		config.AuthKind,
		resolveStaticFunctionAuthConfig(config),
		config.KubeconfigPath,
		config.Namespace)
}

// resolveStaticFunctionAuthConfig builds the fixed auth config for the reverseProxy topology. In basicAuth
// mode the username/password come from the config (the password is injected from a Secret via secretKeyRef).
func resolveStaticFunctionAuthConfig(config *Config) auth.FunctionAuthConfig {
	mode := auth.AuthenticationMode(config.AuthMode)
	if mode == "" {
		mode = auth.AuthenticationModeNone
	}

	return auth.FunctionAuthConfig{
		Mode:              mode,
		BasicAuthUsername: config.BasicAuthUsername,
		BasicAuthPassword: config.BasicAuthPassword,
	}
}
