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
	"time"
)

// AuthTimeout bounds a single auth-url call on the data path so the sidecar fails
// closed quickly instead of inheriting pkg/auth's long (up to 60s) retry window.
const AuthTimeout = 10 * time.Second

// AuthenticationMode is the function level authentication mode
type AuthenticationMode string

const (
	ModeNone      AuthenticationMode = "none"
	ModeAPI       AuthenticationMode = "api"
	ModeBrowser   AuthenticationMode = "browser"
	ModeBasicAuth AuthenticationMode = "basicAuth"
)

// FunctionAuthConfig is the authentication configuration resolved for a single function.
type FunctionAuthConfig struct {
	Mode              AuthenticationMode
	BasicAuthUsername string
	BasicAuthPassword string
}

// Authenticator authenticates an incoming request, resolving the target function from the request itself.
type Authenticator interface {
	// Authenticate returns true if the request is authorized. On false it has already written the
	// mode-appropriate rejection (401, or a 302 to the sign-in URL) to responseWriter.
	Authenticate(responseWriter http.ResponseWriter, request *http.Request) bool
}

// TargetAuthenticator extends Authenticator for callers that operate in authOnly mode and know the target
// function by name rather than by inspecting the request. AuthenticateTarget sets the target header and
// delegates to Authenticate, keeping the caller HTTP-agnostic.
type TargetAuthenticator interface {
	Authenticator
	AuthenticateTarget(functionName string) bool
}
