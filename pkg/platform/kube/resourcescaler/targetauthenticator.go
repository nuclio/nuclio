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

package resourcescaler

import (
	"fmt"
	"io"
	"net/http"

	"github.com/nuclio/nuclio/pkg/auth/authproxy"
	"github.com/nuclio/nuclio/pkg/common/headers"
	"github.com/nuclio/nuclio/pkg/platform/abstract"

	"github.com/nuclio/errors"
	"github.com/nuclio/logger"
	"github.com/v3io/scaler/pkg/scalertypes"
)

// forwardedHeaders are the headers the auth-proxy decides on, copied verbatim from the caller's
// request. The credential itself is what the auth-url validates; the authenticator kind only selects a
// validator, so a caller-supplied kind cannot bypass the check.
var forwardedHeaders = []string{
	headers.AuthorizationHeader,
	headers.CookieHeader,
	headers.IguazioAuthenticatorKind,
}

// AuthOnlyAuthenticator implements scalertypes.TargetAuthenticator, which is what the DLX calls with a
// resolved function name to ask whether a request may start a scaled-to-zero function. The split is
// deliberate: the DLX knows the request and which function it resolved to, the auth-proxy owns the
// authentication logic.
type AuthOnlyAuthenticator struct {
	logger     logger.Logger
	authProxy  string
	httpClient *http.Client
}

// AuthenticateTarget implements scalertypes.TargetAuthenticator by delegating to the co-located
// auth-proxy sidecar.
func (t *AuthOnlyAuthenticator) AuthenticateTarget(res http.ResponseWriter,
	req *http.Request,
	functionName string) bool {
	authResponse, err := t.requestVerdict(req, functionName)
	if err != nil {

		// no verdict at all, so there is nothing to honor - fail closed
		t.logger.WarnWithCtx(req.Context(),
			"Failed to reach the auth-proxy, failing closed",
			"err", err.Error())
		http.Error(res, http.StatusText(http.StatusBadGateway), http.StatusBadGateway)
		return false
	}
	// defer closing the body so that the connection can be reused.
	defer func(Body io.ReadCloser) {
		if err := Body.Close(); err != nil {
			t.logger.DebugWith("Failed to close the auth-proxy response body", "err", err.Error())
		}
	}(authResponse.Body) // nolint: errcheck

	// the auth-proxy answers an allow with exactly 200 and nothing else, so anything unexpected is
	// treated as a rejection rather than guessed at
	if authResponse.StatusCode == http.StatusOK {
		t.logger.DebugWithCtx(req.Context(), "Request authenticated")
		return true
	}

	t.logger.DebugWithCtx(req.Context(),
		"Request rejected by the auth-proxy",
		"statusCode", authResponse.StatusCode)
	t.relayRejection(res, authResponse)
	return false
}

// newAuthOnlyAuthenticator returns the DLX's authentication hook, or nil when function-level
// authentication is disabled platform-wide.
func (n *NuclioResourceScaler) newAuthOnlyAuthenticator() scalertypes.TargetAuthenticator {
	if !n.platformConfiguration.IsFunctionAuthenticationEnabled() {
		return nil
	}

	authProxy := fmt.Sprintf("http://127.0.0.1:%d", abstract.AuthProxyProcessorListenPort)
	n.logger.InfoWith("Function authentication is enabled, DLX will authenticate before scaling from zero",
		"authProxy", authProxy)

	return &AuthOnlyAuthenticator{
		logger:     n.logger.GetChild("auth-only-authenticator"),
		authProxy:  authProxy,
		httpClient: newAuthProxyHTTPClient(),
	}
}

// newAuthProxyHTTPClient builds the client used to query the auth-proxy.
func newAuthProxyHTTPClient() *http.Client {
	return &http.Client{
		// sharing the same timeout for the auth-proxy auth-url call
		Timeout: authproxy.AuthTimeout,

		// A 302 from browser-mode authentication is returned to the caller so the browser can follow the sign-in redirect.
		// It must not be treated as a failure or followed by the DLX, which cannot access the sign-in URL from inside the pod.
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
}

// requestVerdict asks the auth-proxy about req. The caller's method and request line are replayed at
// the auth-proxy's loopback address so it sees the URL the caller actually asked for, which is what a
// browser-mode redirect has to point back at. No body is sent: the decision rests on the headers alone,
// and the caller's body still has to reach the function once it is running.
func (t *AuthOnlyAuthenticator) requestVerdict(req *http.Request,
	functionName string) (*http.Response, error) {
	authRequest, err := http.NewRequestWithContext(req.Context(),
		req.Method,
		t.authProxy+req.URL.RequestURI(),
		nil)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to create auth request")
	}

	for _, headerName := range forwardedHeaders {
		for _, headerValue := range req.Header.Values(headerName) {
			authRequest.Header.Add(headerName, headerValue)
		}
	}

	// the request line carries the path, but its host is now the loopback proxy, so the caller's host
	// travels separately - the two together are what the redirect target is built from
	authRequest.Header.Set(headers.ForwardHost, originalHost(req))
	authRequest.Header.Set(headers.TargetFunctionName, functionName)

	authResponse, err := t.httpClient.Do(authRequest)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to call the auth-proxy")
	}
	return authResponse, nil
}

// relayRejection copies the auth-proxy's rejection to the caller. Location and WWW-Authenticate carry
// the browser redirect target and the basic-auth challenge, so a rejection stripped of them would
// reach the caller as an unactionable error.
func (t *AuthOnlyAuthenticator) relayRejection(res http.ResponseWriter, authResponse *http.Response) {
	for _, headerName := range []string{"Location", "WWW-Authenticate"} {
		for _, headerValue := range authResponse.Header.Values(headerName) {
			res.Header().Add(headerName, headerValue)
		}
	}

	res.WriteHeader(authResponse.StatusCode)
	if _, err := io.Copy(res, authResponse.Body); err != nil {
		t.logger.WarnWith("Failed to relay the rejection body", "err", err.Error())
	}
}

// originalHost returns the host the caller addressed, which is what a browser-mode redirect must point
// back at - the DLX is behind the function's Service, so its own host is not where the caller was going.
func originalHost(req *http.Request) string {
	if forwardedHost := req.Header.Get(headers.ForwardHost); forwardedHost != "" {
		return forwardedHost
	}
	return req.Host
}
