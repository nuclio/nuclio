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
	"time"

	authpkg "github.com/nuclio/nuclio/pkg/auth"
	"github.com/nuclio/nuclio/pkg/common/headers"

	"github.com/nuclio/logger"
)

// AuthTimeout bounds a single auth-url (GetSelf) call on the data path so the sidecar fails
// closed quickly instead of inheriting pkg/auth's long (up to 60s) retry window.
const AuthTimeout = 10 * time.Second

// Authentication modes for a function's authenticationMode (HLD BE Nuclio Function-level Authentication, §2.1.2).
const (
	ModeNone      = "none"
	ModeAPI       = "api"
	ModeBrowser   = "browser"
	ModeBasicAuth = "basicAuth"
)

// Settings is the authentication configuration resolved for a single request.
type Settings struct {
	Mode              string
	BasicAuthUsername string
	BasicAuthPassword string
}

// Authenticator authenticates an incoming request.
type Authenticator interface {
	// Authenticate returns true if the request is authorized. On false it has already written the
	// mode-appropriate rejection (401, or a 302 to the sign-in URL) to responseWriter.
	Authenticate(responseWriter http.ResponseWriter, request *http.Request) bool
}

// decider is the shared decision core used by every topology. Given resolved Settings it verifies
// basicAuth locally, calls the auth-url for api/browser, and always fails closed on error.
type decider struct {
	logger    logger.Logger
	auth      authpkg.Auth
	signinURL string
}

// decide applies the verdict for the resolved settings, writing the rejection response itself on failure.
func (d *decider) decide(responseWriter http.ResponseWriter, request *http.Request, settings Settings) bool {
	switch settings.Mode {
	case ModeNone:
		return true
	case ModeBasicAuth:
		return d.verifyBasicAuth(responseWriter, request, settings)
	case ModeAPI:
		return d.callAuthURL(responseWriter, request, false)
	case ModeBrowser:
		return d.callAuthURL(responseWriter, request, true)
	default:
		d.logger.WarnWithCtx(request.Context(),
			"Unknown authentication mode, failing closed",
			"mode", settings.Mode)
		http.Error(responseWriter, "Forbidden", http.StatusForbidden)
		return false
	}
}

// verifyBasicAuth checks HTTP Basic credentials locally (never delegated to the auth-url).
func (d *decider) verifyBasicAuth(responseWriter http.ResponseWriter, request *http.Request, settings Settings) bool {
	username, password, ok := request.BasicAuth()
	if ok &&
		subtle.ConstantTimeCompare([]byte(username), []byte(settings.BasicAuthUsername)) == 1 &&
		subtle.ConstantTimeCompare([]byte(password), []byte(settings.BasicAuthPassword)) == 1 {
		return true
	}

	responseWriter.Header().Set("WWW-Authenticate", `Basic realm="Authentication Required"`)
	http.Error(responseWriter, "Unauthorized", http.StatusUnauthorized)
	return false
}

// callAuthURL validates the credential against the auth-url (GetSelf) with a bounded timeout, failing
// closed on any error: 401 for api, 302 to the sign-in URL for browser.
func (d *decider) callAuthURL(responseWriter http.ResponseWriter, request *http.Request, browser bool) bool {
	if d.auth == nil {
		d.logger.WarnWithCtx(request.Context(), "Auth endpoint is not configured, failing closed")
		d.reject(responseWriter, request, browser)
		return false
	}

	ctx, cancel := context.WithTimeout(request.Context(), AuthTimeout)
	defer cancel()

	session, err := d.auth.Authenticate(request.WithContext(ctx), &authpkg.Options{})
	if err != nil {
		d.logger.WarnWithCtx(ctx,
			"Authentication failed, rejecting request",
			"browser", browser,
			"err", err.Error())
		d.reject(responseWriter, request, browser)
		return false
	}

	d.applyIdentityHeaders(request, session)
	return true
}

// reject writes the mode-appropriate rejection response.
func (d *decider) reject(responseWriter http.ResponseWriter, request *http.Request, browser bool) {
	if browser {
		http.Redirect(responseWriter, request, d.buildSigninRedirect(request), http.StatusFound)
		return
	}
	http.Error(responseWriter, "Unauthorized", http.StatusUnauthorized)
}

// applyIdentityHeaders forwards the authenticated identity to the upstream on the request headers.
func (d *decider) applyIdentityHeaders(request *http.Request, session authpkg.Session) {
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
func (d *decider) buildSigninRedirect(request *http.Request) string {
	parsed, err := url.Parse(d.signinURL)
	if err != nil {
		return d.signinURL
	}
	query := parsed.Query()
	query.Set("rd", originalURL(request))
	parsed.RawQuery = query.Encode()
	return parsed.String()
}

// originalURL reconstructs the externally requested URL, honoring X-Forwarded-* set by the ingress.
func originalURL(request *http.Request) string {
	scheme := "https"
	if proto := request.Header.Get("X-Forwarded-Proto"); proto != "" {
		scheme = proto
	}
	host := request.Host
	if forwardedHost := request.Header.Get("X-Forwarded-Host"); forwardedHost != "" {
		host = forwardedHost
	}
	return fmt.Sprintf("%s://%s%s", scheme, host, request.URL.RequestURI())
}
