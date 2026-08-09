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

	"github.com/nuclio/errors"
)

// Config holds the auth-proxy sidecar configuration.
type Config struct {
	Mode               auth.ProxyMode
	Routes             []authproxy.Route // every port the auth-proxy listens on, each with its own upstream
	AuthURL            string            // the URL of the auth service to which requests are sent for authentication
	SigninURL          string            // the URL of the sign-in service to which requests are redirected for sign-in (browser mode only)
	AuthMode           string
	FunctionConfigPath string // path to the mounted function config; credentials are read from it when auth-mode=basicAuth
	Namespace          string
	KubeconfigPath     string
	PlatformConfigPath string
	AuthKind           auth.Kind // auth kind used for API/browser authentication
}

func validateConfiguration(config *Config) error {
	if err := validateRoutes(config); err != nil {
		return errors.Wrap(err, "Invalid route configuration")
	}

	switch config.Mode {
	case auth.ProxyModeReverseProxy:
		return validateReverseProxyConfiguration(config)
	case auth.ProxyModeAuthOnly:
		return validateAuthOnlyConfiguration(config)
	default:
		return errors.Errorf("Unknown auth-proxy mode: %s", config.Mode)
	}
}

func validateReverseProxyConfiguration(config *Config) error {
	switch auth.AuthenticationMode(config.AuthMode) {
	case auth.AuthenticationModeAPI:
		if config.AuthURL == "" {
			return errors.New("Auth URL must be provided for 'api' authentication mode")
		}
	case auth.AuthenticationModeBrowser:
		if config.AuthURL == "" {
			return errors.New("Auth URL must be provided for 'browser' authentication mode")
		}
		if config.SigninURL == "" {
			return errors.New("Sign-in URL must be provided for 'browser' authentication mode")
		}
	case auth.AuthenticationModeBasicAuth:
		if config.FunctionConfigPath == "" {
			return errors.New("Function config path must be provided for 'basicAuth' authentication mode")
		}
	case auth.AuthenticationModeNone, "":
		// no additional requirements
	default:
		return errors.Errorf("Unknown authentication mode: %s", config.AuthMode)
	}

	return nil
}

func validateAuthOnlyConfiguration(config *Config) error {
	if config.Namespace == "" {
		return errors.New("Namespace must be provided in authOnly mode")
	}
	if config.AuthURL == "" {
		return errors.New("Auth URL must be provided in authOnly mode")
	}
	if config.SigninURL == "" {
		return errors.New("Sign-in URL must be provided in authOnly mode")
	}

	return nil
}

func validateRoutes(config *Config) error {
	if len(config.Routes) == 0 {
		return errors.New("At least one route must be provided")
	}

	seenListenPorts := make(map[int]bool, len(config.Routes))
	for _, route := range config.Routes {
		if err := validatePort(route.ListenPort); err != nil {
			return err
		}

		if seenListenPorts[route.ListenPort] {
			return errors.Errorf("Duplicate listen port: %d", route.ListenPort)
		}
		seenListenPorts[route.ListenPort] = true
	}

	switch config.Mode {
	case auth.ProxyModeReverseProxy:

		// every listener forwards, so each route needs its own upstream
		for _, route := range config.Routes {
			if route.UpstreamURL == "" {
				return errors.Errorf("Upstream URL must be provided for listen port: %d", route.ListenPort)
			}
		}
	case auth.ProxyModeAuthOnly:

		// authOnly is only ever reached over a single, internal loopback address
		if len(config.Routes) != 1 {
			return errors.Errorf("authOnly mode requires exactly one route, got %d", len(config.Routes))
		}

		// authOnly does not forward, so an upstream is always a configuration mistake
		if config.Routes[0].UpstreamURL != "" {
			return errors.New("Upstream URL must not be provided in authOnly mode")
		}
	}

	return nil
}

func validatePort(listenPort int) error {
	// TCP ports are 16-bit unsigned integers, so the valid range is 1-65535 (0 is reserved)
	if listenPort < 1 || listenPort > 65535 {
		return errors.Errorf("Invalid listen port: %d", listenPort)
	}

	// ports 1 through 1023 are known as privileged ports or well-known ports, so we should avoid using them
	if listenPort < 1024 {
		return errors.Errorf("Listen port is reserved for well-known services; please use a port above 1023; invalid port: %d", listenPort)
	}

	return nil
}
