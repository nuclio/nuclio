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

	"github.com/nuclio/errors"
)

// Config holds the auth-proxy sidecar configuration.
type Config struct {
	Mode               auth.ProxyMode
	ListenPort         int
	UpstreamURL        string // the URL of the upstream service to which requests are proxied (reverseProxy mode only)
	AuthURL            string // the URL of the auth service to which requests are sent for authentication
	SigninURL          string // the URL of the sign-in service to which requests are redirected for sign-in (browser mode only)
	AuthMode           string
	BasicAuthUsername  string
	BasicAuthPassword  string
	Namespace          string
	KubeconfigPath     string
	PlatformConfigPath string
	AuthKind           auth.Kind // auth kind used for API/browser authentication
}

func validateConfiguration(config *Config) error {
	if err := validatePorts(config); err != nil {
		return errors.Wrap(err, "Invalid port configuration")
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
	if config.UpstreamURL == "" {
		return errors.New("Upstream URL must be provided")
	}

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
		if config.BasicAuthUsername == "" {
			return errors.New("Basic-auth username must be provided for 'basicAuth' authentication mode")
		}
		if config.BasicAuthPassword == "" {
			return errors.New("Basic-auth password must be provided for 'basicAuth' authentication mode")
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

func validatePorts(config *Config) error {
	// TCP ports are 16-bit unsigned integers, so the valid range is 1-65535 (0 is reserved)
	if config.ListenPort < 1 || config.ListenPort > 65535 {
		return errors.Errorf("Invalid listen port: %d", config.ListenPort)
	}

	// ports 1 through 1023 are known as privileged ports or well-known ports, so we should avoid using them
	if config.ListenPort < 1024 {
		return errors.Errorf("Listen port is reserved for well-known services; please use a port above 1023; invalid port: %d", config.ListenPort)
	}

	return nil
}
