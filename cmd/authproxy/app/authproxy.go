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
	"os"
	"strings"

	"github.com/nuclio/nuclio/pkg/auth/authproxy"
	"github.com/nuclio/nuclio/pkg/loggersink"
	"github.com/nuclio/nuclio/pkg/platformconfig"

	"github.com/nuclio/errors"
	"github.com/nuclio/logger"
	"golang.org/x/crypto/bcrypt"

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
	staticConfig, err := resolveStaticFunctionAuthConfig(config)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to resolve static function auth config")
	}
	return authproxy.NewAuthenticator(rootLogger,
		config.Mode,
		config.AuthURL,
		config.SigninURL,
		config.AuthKind,
		staticConfig,
		config.KubeconfigPath,
		config.Namespace)
}

// resolveStaticFunctionAuthConfig builds the fixed auth config for the reverseProxy topology.
// For basicAuth mode it reads the password from the Secret-volume file, bcrypt-hashes it, and stores
// only the hash in FunctionAuthConfig so plaintext never enters that struct.
func resolveStaticFunctionAuthConfig(config *Config) (authproxy.FunctionAuthConfig, error) {
	mode := authproxy.AuthenticationMode(config.AuthMode)
	if mode == "" {
		mode = authproxy.ModeNone
	}

	functionAuthConfig := authproxy.FunctionAuthConfig{
		Mode:              mode,
		BasicAuthUsername: config.BasicAuthUsername,
	}

	if config.BasicAuthPasswordPath != "" {
		data, err := os.ReadFile(config.BasicAuthPasswordPath)
		if err != nil {
			return authproxy.FunctionAuthConfig{}, errors.Wrapf(err, "Failed to read basic-auth password file '%s'", config.BasicAuthPasswordPath)
		}
		hash, err := bcrypt.GenerateFromPassword([]byte(strings.TrimSpace(string(data))), bcrypt.DefaultCost)
		if err != nil {
			return authproxy.FunctionAuthConfig{}, errors.Wrap(err, "Failed to hash basic-auth password")
		}
		functionAuthConfig.BasicAuthPasswordHash = hash
		config.BasicAuthPasswordPath = ""
	}

	return functionAuthConfig, nil
}
