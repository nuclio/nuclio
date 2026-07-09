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
	"fmt"
	"net/http"

	"github.com/nuclio/nuclio/pkg/auth"
	"github.com/nuclio/nuclio/pkg/loggersink"
	"github.com/nuclio/nuclio/pkg/platformconfig"

	"github.com/nuclio/errors"
	"github.com/nuclio/logger"

	// register logger sinks (stdout, appinsights) into the loggersink registry via init()
	_ "github.com/nuclio/nuclio/pkg/sinks"
)

// Run creates and starts the auth-proxy server for the given mode.
func Run(mode auth.ProxyMode,
	listenPort int,
	upstreamURL string,
	authURL string,
	signinURL string,
	platformConfigurationPath string) error {
	if err := validateConfiguration(listenPort, upstreamURL, authURL, signinURL); err != nil {
		return errors.Wrap(err, "Invalid auth-proxy configuration")
	}

	platformConfiguration, err := platformconfig.NewPlatformConfig(platformConfigurationPath)
	if err != nil {
		return errors.Wrap(err, "Failed to get platform configuration")
	}

	rootLogger, err := loggersink.CreateSystemLogger("auth-proxy", platformConfiguration)
	if err != nil {
		return errors.Wrap(err, "Failed to create logger")
	}

	server, err := newServer(rootLogger, mode, listenPort, upstreamURL, authURL, signinURL)
	if err != nil {
		return errors.Wrap(err, "Failed to create auth-proxy server")
	}
	if err := server.start(); err != nil {
		return errors.Wrap(err, "Failed to start auth-proxy server")
	}
	select {}
}

func validateConfiguration(listenPort int, upstreamURL string, authURL string, signinURL string) error {
	// TCP ports are 16-bit unsigned integers, so the valid range is 1-65535 (0 is reserved)
	if listenPort < 1 || listenPort > 65535 {
		return errors.Errorf("Invalid listen port: %d", listenPort)
	}

	if upstreamURL == "" {
		return errors.New("Upstream URL must be provided")
	}

	if authURL == "" {
		return errors.New("Auth URL must be provided")
	}

	if signinURL == "" {
		return errors.New("Redirect URL must be provided")
	}

	return nil
}
