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

package main

import (
	"flag"
	"os"
	"strconv"
	"strings"

	"github.com/nuclio/nuclio/cmd/authproxy/app"
	"github.com/nuclio/nuclio/pkg/auth"
	"github.com/nuclio/nuclio/pkg/common"

	"github.com/nuclio/errors"
)

func main() {
	mode := flag.String("mode", common.GetEnvOrDefaultString("NUCLIO_AUTHPROXY_MODE", string(auth.ProxyModeReverseProxy)), "Auth-proxy mode: reverseProxy or authOnly")
	listenPort := flag.String("listen-port", common.GetEnvOrDefaultString("NUCLIO_AUTHPROXY_LISTEN_PORT", "8080"), "Port the auth-proxy listens on")
	upstreamURL := flag.String("upstream-url", common.GetEnvOrDefaultString("NUCLIO_AUTHPROXY_UPSTREAM_URL", "http://127.0.0.1:6080"), "URL of the upstream service (processor) to forward requests to")
	authURL := flag.String("auth-url", common.GetEnvOrDefaultString("NUCLIO_AUTHPROXY_AUTH_URL", ""), "URL of the authentication endpoint")
	signinURL := flag.String("signin-url", common.GetEnvOrDefaultString("NUCLIO_AUTHPROXY_SIGNIN_URL", ""), "URL unauthenticated browser requests are redirected to sign-in")
	authMode := flag.String("auth-mode", common.GetEnvOrDefaultString("NUCLIO_AUTHPROXY_AUTH_MODE", string(auth.AuthenticationModeNone)), "Authentication mode for reverseProxy: none, api, browser or basicAuth")
	functionConfigPath := flag.String("function-config", common.GetEnvOrDefaultString("NUCLIO_AUTHPROXY_FUNCTION_CONFIG_PATH", "/etc/nuclio/config/processor/processor.yaml"), "Path to the mounted function configuration file (used only when auth-mode=basicAuth)")
	namespace := flag.String("namespace", common.GetEnvOrDefaultString("NUCLIO_AUTHPROXY_NAMESPACE", ""), "Namespace of the target functions")
	kubeconfigPath := flag.String("kubeconfig-path", os.Getenv("KUBECONFIG"), "Path of kubeconfig file")
	platformConfigurationPath := flag.String("platform-config", "/etc/nuclio/config/platform/platform.yaml", "Path of platform configuration file")
	authKind := flag.String("auth-kind", auth.KindNop, "Authentication kind for API/browser authentication (e.g. iguazio, iguazio-v4, nop)")
	flag.Parse()

	listenPortInt, err := strconv.Atoi(strings.TrimSpace(*listenPort))
	if err != nil {
		errors.PrintErrorStack(os.Stderr, errors.Wrap(err, "Failed to parse listen port"), 5)
		os.Exit(1)
	}

	if err := app.Run(&app.Config{
		Mode:               auth.ProxyMode(*mode),
		ListenPort:         listenPortInt,
		UpstreamURL:        *upstreamURL,
		AuthURL:            *authURL,
		SigninURL:          *signinURL,
		AuthMode:           *authMode,
		FunctionConfigPath: *functionConfigPath,
		Namespace:          *namespace,
		KubeconfigPath:     *kubeconfigPath,
		PlatformConfigPath: *platformConfigurationPath,
		AuthKind:           auth.Kind(*authKind),
	}); err != nil {
		errors.PrintErrorStack(os.Stderr, err, 5)
		os.Exit(1)
	}
}
