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

	"github.com/nuclio/nuclio/cmd/authproxy/app"
	"github.com/nuclio/nuclio/pkg/auth"
	"github.com/nuclio/nuclio/pkg/common"

	"github.com/nuclio/errors"
)

func main() {
	mode := flag.String("mode", common.GetEnvOrDefaultString("NUCLIO_AUTHPROXY_MODE", string(auth.ProxyModeReverseProxy)), "Auth-proxy mode: reverseProxy or authOnly")
	listenPort := flag.Int("listen-port", common.GetEnvOrDefaultInt("NUCLIO_AUTHPROXY_LISTEN_PORT", 8080), "Port the auth-proxy listens on")
	upstreamURL := flag.String("upstream-url", common.GetEnvOrDefaultString("NUCLIO_AUTHPROXY_UPSTREAM_URL", "http://127.0.0.1:6080"), "URL of the upstream service (processor) to forward requests to")
	authURL := flag.String("auth-url", common.GetEnvOrDefaultString("NUCLIO_AUTHPROXY_AUTH_URL", ""), "URL of the authentication endpoint")
	signinURL := flag.String("signin-url", common.GetEnvOrDefaultString("NUCLIO_AUTHPROXY_SIGNIN_URL", ""), "URL unauthenticated browser requests are redirected to sign-in")
	authMode := flag.String("auth-mode", common.GetEnvOrDefaultString("NUCLIO_AUTHPROXY_AUTH_MODE", "none"), "Authentication mode for reverseProxy: none, api, browser or basicAuth")
	basicAuthUsername := flag.String("basic-auth-username", common.GetEnvOrDefaultString("NUCLIO_AUTHPROXY_BASIC_AUTH_USERNAME", ""), "Basic-auth username (used only when auth-mode=basicAuth)")
	basicAuthPassword := flag.String("basic-auth-password", common.GetEnvOrDefaultString("NUCLIO_AUTHPROXY_BASIC_AUTH_PASSWORD", ""), "Basic-auth password, injected from a Secret via secretKeyRef (used only when auth-mode=basicAuth)")
	namespace := flag.String("namespace", common.GetEnvOrDefaultString("NUCLIO_AUTHPROXY_NAMESPACE", ""), "Namespace of the target functions")
	kubeconfigPath := flag.String("kubeconfig-path", os.Getenv("KUBECONFIG"), "Path of kubeconfig file")
	platformConfigurationPath := flag.String("platform-config", "/etc/nuclio/config/platform/platform.yaml", "Path of platform configuration file")
	flag.Parse()

	if err := app.Run(&app.Config{
		Mode:               auth.ProxyMode(*mode),
		ListenPort:         *listenPort,
		UpstreamURL:        *upstreamURL,
		AuthURL:            *authURL,
		SigninURL:          *signinURL,
		AuthMode:           *authMode,
		BasicAuthUsername:  *basicAuthUsername,
		BasicAuthPassword:  *basicAuthPassword,
		Namespace:          *namespace,
		KubeconfigPath:     *kubeconfigPath,
		PlatformConfigPath: *platformConfigurationPath,
	}); err != nil {
		errors.PrintErrorStack(os.Stderr, err, 5)
		os.Exit(1)
	}
}
