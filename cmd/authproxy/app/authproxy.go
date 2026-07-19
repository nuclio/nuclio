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
	authpkg "github.com/nuclio/nuclio/pkg/auth"
	"github.com/nuclio/nuclio/pkg/auth/authproxy"
	"github.com/nuclio/nuclio/pkg/common"
	"github.com/nuclio/nuclio/pkg/loggersink"
	kubeclient "github.com/nuclio/nuclio/pkg/platform/kube/clients/kube"
	nuclioioclient "github.com/nuclio/nuclio/pkg/platform/kube/clients/nuclio/clientset/versioned"
	"github.com/nuclio/nuclio/pkg/platformconfig"

	"github.com/nuclio/errors"
	"github.com/nuclio/logger"

	// register logger sinks (stdout, appinsights) into the loggersink registry via init()
	_ "github.com/nuclio/nuclio/pkg/sinks"
)

// Config holds the auth-proxy sidecar configuration.
type Config struct {
	Mode               authpkg.ProxyMode
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
}

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

// newAuthenticator builds the topology-appropriate authenticator; each constructor wires its own
// shared decision core (auth-url validator, sign-in URL) from the config.
func newAuthenticator(rootLogger logger.Logger, config *Config) (authproxy.Authenticator, error) {
	switch config.Mode {
	case authpkg.ProxyModeReverseProxy:
		return authproxy.NewReverseProxyAuthenticator(rootLogger,
			config.AuthURL,
			config.SigninURL,
			resolveStaticFunctionAuthConfig(config)), nil
	case authpkg.ProxyModeAuthOnly:
		clientConfig, err := common.GetClientConfig(config.KubeconfigPath)
		if err != nil {
			return nil, errors.Wrap(err, "Failed to get client configuration")
		}
		nuclioClientSet, err := nuclioioclient.NewForConfig(clientConfig)
		if err != nil {
			return nil, errors.Wrap(err, "Failed to create nuclio client set")
		}
		kubeClientSet, err := kubeclient.NewClientWithRetryFromConfig(clientConfig)
		if err != nil {
			return nil, errors.Wrap(err, "Failed to create kube client set")
		}
		return authproxy.NewAuthOnlyAuthenticator(rootLogger,
			config.AuthURL,
			config.SigninURL,
			nuclioClientSet,
			kubeClientSet,
			config.Namespace), nil
	default:
		return nil, errors.Errorf("Unknown auth-proxy mode: %s", config.Mode)
	}
}

// resolveStaticFunctionAuthConfig builds the fixed auth config for the reverseProxy topology. In basicAuth
// mode the username/password come from the config (the password is injected from a Secret via secretKeyRef).
func resolveStaticFunctionAuthConfig(config *Config) authproxy.FunctionAuthConfig {
	mode := authproxy.AuthenticationMode(config.AuthMode)
	if mode == "" {
		mode = authproxy.ModeNone
	}

	return authproxy.FunctionAuthConfig{
		Mode:              mode,
		BasicAuthUsername: config.BasicAuthUsername,
		BasicAuthPassword: config.BasicAuthPassword,
	}
}

func validateConfiguration(config *Config) error {
	// TCP ports are 16-bit unsigned integers, so the valid range is 1-65535 (0 is reserved)
	if config.ListenPort < 1 || config.ListenPort > 65535 {
		return errors.Errorf("Invalid listen port: %d", config.ListenPort)
	}

	switch config.Mode {
	case authpkg.ProxyModeReverseProxy:
		return validateReverseProxyConfiguration(config)
	case authpkg.ProxyModeAuthOnly:
		return validateAuthOnlyConfiguration(config)
	default:
		return errors.Errorf("Unknown auth-proxy mode: %s", config.Mode)
	}
}

func validateReverseProxyConfiguration(config *Config) error {
	if config.UpstreamURL == "" {
		return errors.New("Upstream URL must be provided")
	}

	switch authproxy.AuthenticationMode(config.AuthMode) {
	case authproxy.ModeAPI:
		if config.AuthURL == "" {
			return errors.New("Auth URL must be provided for 'api' authentication mode")
		}
	case authproxy.ModeBrowser:
		if config.AuthURL == "" {
			return errors.New("Auth URL must be provided for 'browser' authentication mode")
		}
		if config.SigninURL == "" {
			return errors.New("Sign-in URL must be provided for 'browser' authentication mode")
		}
	case authproxy.ModeBasicAuth:
		if config.BasicAuthUsername == "" {
			return errors.New("Basic-auth username must be provided for 'basicAuth' authentication mode")
		}
		if config.BasicAuthPassword == "" {
			return errors.New("Basic-auth password must be provided for 'basicAuth' authentication mode")
		}
	case authproxy.ModeNone, "":

		// no additional requirements
	default:
		return errors.Errorf("Unknown authentication mode: %s", config.AuthMode)
	}

	return nil
}

func validateAuthOnlyConfiguration(config *Config) error {

	// authOnly resolves the mode per request from the CRD, so any mode is possible and auth-url /
	// sign-in URL / namespace must all be available
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
