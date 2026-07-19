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

package authproxy

import (
	"net/http"

	"github.com/nuclio/logger"
)

// reverseProxyAuthenticator serves the running-function (reverseProxy) topology: its auth config is
// rendered into the pod once (per function), so every request uses the same resolved FunctionAuthConfig.
type reverseProxyAuthenticator struct {
	*abstractAuthenticator
	authConfig FunctionAuthConfig
}

// NewReverseProxyAuthenticator creates an Authenticator with a fixed FunctionAuthConfig (function-pod topology).
func NewReverseProxyAuthenticator(parentLogger logger.Logger,
	authURL string,
	signinURL string,
	authConfig FunctionAuthConfig) Authenticator {
	parentLogger.InfoWith("Creating reverse-proxy authenticator", "authURL", authURL, "signinURL", signinURL, "authConfig", authConfig)
	return &reverseProxyAuthenticator{
		abstractAuthenticator: newAbstractAuthenticator(parentLogger, authURL, signinURL),
		authConfig:            authConfig,
	}
}

func (a *reverseProxyAuthenticator) Authenticate(responseWriter http.ResponseWriter, request *http.Request) bool {
	return a.decide(responseWriter, request, a.authConfig)
}
