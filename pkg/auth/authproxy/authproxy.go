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

// Package authproxy holds the auth-proxy sidecar's decision logic, shared by both topologies:
// the running-function reverse proxy (static settings) and the DLX-side authOnly endpoint
// (settings resolved per request from the function CRD).
package authproxy

import (
	"context"
	"crypto/subtle"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	authpkg "github.com/nuclio/nuclio/pkg/auth"
	"github.com/nuclio/nuclio/pkg/auth/factory"
	"github.com/nuclio/nuclio/pkg/common/headers"

	"github.com/nuclio/errors"
	"github.com/nuclio/logger"
)

// newAuthInstance creates an auth client for the given kind, configured against authURL.
// Returns an error for kinds that do not use Iguazio-style URL verification (e.g. KindNop).
func newAuthInstance(parentLogger logger.Logger, authURL string, authKind authpkg.Kind) (authpkg.Auth, error) {
	authConfig := authpkg.NewConfig(authKind)
	switch authKind {
	case authpkg.KindIguazio, authpkg.KindIguazioV4:
		if authConfig.Iguazio == nil {
			return nil, errors.Errorf("auth kind %q does not support URL-based verification", authKind)
		}
		authConfig.Iguazio.VerificationURL = authURL
		authConfig.Iguazio.Timeout = AuthTimeout
	default:
	}
	return factory.NewAuth(parentLogger, authConfig), nil
}

// abstractAuthenticator holds the shared authentication logic embedded by every topology. Given a resolved
// FunctionAuthConfig it verifies basicAuth locally, calls the auth-url for api/browser, and always fails closed on error.
type abstractAuthenticator struct {
	logger    logger.Logger
	auth      authpkg.Auth
	signinURL *url.URL
}

// newAbstractAuthenticator builds the shared decision logic. authURL is guaranteed non-empty by the
// caller for any mode that delegates to the auth-url (api/browser); basicAuth/none modes never reach
// callAuthURL so the auth instance is effectively unused for them.
func newAbstractAuthenticator(parentLogger logger.Logger, authURL, signinURL string, authKind authpkg.Kind) (*abstractAuthenticator, error) {
	parsed, err := url.Parse(signinURL)
	if err != nil {
		return nil, errors.Wrapf(err, "Failed to parse sign-in URL")
	}
	authInstance, err := newAuthInstance(parentLogger, authURL, authKind)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to create auth instance")
	}
	return &abstractAuthenticator{
		logger:    parentLogger,
		auth:      authInstance,
		signinURL: parsed,
	}, nil
}

// decide applies the verdict for the resolved authConfig, writing the rejection response itself on failure.
func (a *abstractAuthenticator) decide(responseWriter http.ResponseWriter, request *http.Request, authConfig FunctionAuthConfig) bool {
	switch authConfig.Mode {
	case ModeNone:
		a.logger.InfoWith("Authentication disabled, allowing request", "path", request.URL.Path)
		return true
	case ModeBasicAuth:
		return a.verifyBasicAuth(responseWriter, request, authConfig)
	case ModeAPI:
		return a.callAuthURL(responseWriter, request, false)
	case ModeBrowser:
		return a.callAuthURL(responseWriter, request, true)
	default:
		a.logger.WarnWith("Unknown authentication mode, failing closed",
			"mode", authConfig.Mode)
		http.Error(responseWriter, http.StatusText(http.StatusForbidden), http.StatusForbidden)
		return false
	}
}

// verifyBasicAuth checks HTTP Basic credentials locally (never delegated to the auth-url).
func (a *abstractAuthenticator) verifyBasicAuth(responseWriter http.ResponseWriter, request *http.Request, authConfig FunctionAuthConfig) bool {
	username, password, ok := request.BasicAuth()
	if ok &&
		subtle.ConstantTimeCompare([]byte(username), []byte(authConfig.BasicAuthUsername)) == 1 &&
		subtle.ConstantTimeCompare([]byte(password), []byte(authConfig.BasicAuthPassword)) == 1 {
		return true
	}

	responseWriter.Header().Set("WWW-Authenticate", `Basic realm="Authentication Required"`)
	http.Error(responseWriter, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
	return false
}

// callAuthURL validates the credential against the auth-url with a bounded timeout, failing
// closed on any error: 401 for api, 302 to the sign-in URL for browser.
func (a *abstractAuthenticator) callAuthURL(responseWriter http.ResponseWriter, request *http.Request, browser bool) bool {
	ctx, cancel := context.WithTimeout(request.Context(), AuthTimeout)
	defer cancel()

	session, err := a.auth.Authenticate(request.WithContext(ctx), &authpkg.Options{})
	if err != nil {
		a.logger.WarnWithCtx(ctx,
			"Authentication failed, rejecting request",
			"browser", browser,
			"err", err.Error())
		a.reject(responseWriter, request, browser)
		return false
	}

	a.applyIdentityHeaders(request, session)
	return true
}

// reject writes the mode-appropriate rejection response.
func (a *abstractAuthenticator) reject(responseWriter http.ResponseWriter, request *http.Request, browser bool) {
	if browser {
		a.logger.InfoWith("redirecting to sign-in URL", "path", request.URL.Path, "signinURL", a.signinURL.String())
		http.Redirect(responseWriter, request, a.buildSigninRedirect(request), http.StatusFound)
		return
	}
	http.Error(responseWriter, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
}

// applyIdentityHeaders forwards the authenticated identity to the upstream on the request headers.
func (a *abstractAuthenticator) applyIdentityHeaders(request *http.Request, session authpkg.Session) {
	if session == nil {
		return
	}
	if username := session.GetUsername(); username != "" {
		request.Header.Set(headers.RemoteUser, username)
	}
	if userID := session.GetUserID(); userID != "" {
		request.Header.Set(headers.UserID, userID)
	}
	if groupIDs := session.GetGroupIDs(); len(groupIDs) > 0 {
		request.Header.Set(headers.UserGroupIds, strings.Join(groupIDs, ","))
	}
}

// buildSigninRedirect returns the sign-in URL with the original request URL attached as the rd query param.
// The URL is parsed once at construction time; here we clone it to avoid mutating the cached value.
func (a *abstractAuthenticator) buildSigninRedirect(request *http.Request) string {
	cloned := *a.signinURL
	query := cloned.Query()
	query.Set("rd", originalURL(request))
	cloned.RawQuery = query.Encode()
	return cloned.String()
}

// originalURL reconstructs the externally requested URL for use as the rd= redirect target.
// The scheme is always https: browser mode requires Secure cookies, which browsers only send
// over HTTPS regardless of the proxy or ingress in front.
func originalURL(request *http.Request) string {
	host := request.Host
	if forwardedHost := request.Header.Get(headers.ForwardHost); forwardedHost != "" {
		host = forwardedHost
	}
	return fmt.Sprintf("https://%s%s", host, request.URL.RequestURI())
}
