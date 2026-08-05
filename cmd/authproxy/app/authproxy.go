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
	"github.com/nuclio/nuclio/pkg/common"
	"github.com/nuclio/nuclio/pkg/functionconfig"
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

	return newServer(rootLogger, listenAddress, handler).start()
}

// newAuthenticator creates kube clients when needed and delegates to the pkg-level factory.
func newAuthenticator(rootLogger logger.Logger, config *Config) (authproxy.Authenticator, error) {
	staticAuthConfig, err := resolveStaticFunctionAuthConfig(rootLogger, config)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to resolve function auth config")
	}
	return authproxy.NewAuthenticator(rootLogger,
		config.Mode,
		config.AuthURL,
		config.SigninURL,
		config.AuthKind,
		staticAuthConfig,
		config.KubeconfigPath,
		config.Namespace)
}

// resolveStaticFunctionAuthConfig builds the fixed auth config for the reverseProxy topology. In basicAuth
// mode the username/password are read from the mounted function config file (restored from the secret).
func resolveStaticFunctionAuthConfig(parentLogger logger.Logger, config *Config) (auth.FunctionAuthConfig, error) {
	mode := auth.AuthenticationMode(config.AuthMode)
	if mode == "" {
		mode = auth.AuthenticationModeNone
	}

	if mode != auth.AuthenticationModeBasicAuth {
		return auth.FunctionAuthConfig{Mode: mode}, nil
	}

	username, password, err := readBasicAuthCredentials(parentLogger, config.FunctionConfigPath)
	if err != nil {
		return auth.FunctionAuthConfig{}, errors.Wrap(err, "Failed to read basic-auth credentials from function config")
	}
	return auth.FunctionAuthConfig{
		Mode:              mode,
		BasicAuthUsername: username,
		BasicAuthPassword: password,
	}, nil
}

// readBasicAuthCredentials reads the function config from configPath, restores scrubbed values from the
// mounted secret (when masking is enabled and NUCLIO_RESTORE_FUNCTION_CONFIG_FROM_SECRET is set), and
// extracts the basicAuth username and password from the single HTTP trigger.
func readBasicAuthCredentials(parentLogger logger.Logger, configPath string) (string, string, error) {
	functionConfig, err := functionconfig.ReadConfigFromFile(configPath)
	if err != nil {
		return "", "", errors.Wrap(err, "Failed to read function configuration")
	}

	// Sensitive fields are scrubbed to "$ref:" placeholders unless masking is explicitly disabled.
	// Only restore from the mounted Secret when scrubbing is active; otherwise credentials are already plaintext.
	if !functionConfig.Spec.DisableSensitiveFieldsMasking {
		if restoreFromSecret := common.GetEnvOrDefaultBool(common.RestoreConfigFromSecretEnvVar, false); restoreFromSecret {
			scrubber := functionconfig.NewScrubber(parentLogger, nil, nil)
			secretsMap, err := scrubber.LoadSecretsMap()
			if err != nil {
				return "", "", errors.Wrap(err, "Failed to load secrets map")
			}
			if len(secretsMap) == 0 {
				return "", "", errors.New("Secrets map is empty, cannot restore masked function config credentials")
			}
			restoredInterface, err := scrubber.Restore(functionConfig, secretsMap)
			if err != nil {
				return "", "", errors.Wrap(err, "Failed to restore function config from secret")
			}
			functionConfig = functionconfig.GetFunctionConfigFromInterface(restoredInterface)
			if functionConfig == nil {
				return "", "", errors.New("Failed to convert restored function config")
			}
		}
	}

	trigger, err := functionconfig.GetHTTPTrigger(functionConfig.Spec.Triggers)
	if err != nil {
		return "", "", errors.Wrap(err, "Failed to get HTTP trigger from function config")
	}

	authConfig, err := auth.FunctionAuthConfigFromAttributes(trigger.Attributes, auth.AuthenticationModeBasicAuth)
	if err != nil {
		return "", "", errors.Wrap(err, "Failed to read basicAuth credentials from HTTP trigger")
	}
	return authConfig.BasicAuthUsername, authConfig.BasicAuthPassword, nil
}
