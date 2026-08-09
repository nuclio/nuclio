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
//
// A slice, not a set: these are iterated to pull each one out of the caller's request, so the cost is
// one map lookup per name. Testing set membership while walking the caller's headers instead would cost
// a lookup per header the caller happened to send, which is the larger number.
var forwardedHeaders = []string{
	headers.AuthorizationHeader,
	headers.CookieHeader,
	headers.IguazioAuthenticatorKind,
}

// AuthOnlyAuthenticator asks the auth-proxy co-located in the DLX pod whether a request may start a
// scaled-to-zero function. The split is deliberate: the DLX knows the request and which function it
// resolved to, the auth-proxy owns every bit of authentication logic. This type holds none - it
// forwards the request, names the target, and relays whatever verdict comes back.
//
// It implements scalertypes.TargetAuthenticator, which is what the DLX calls: the target arrives as a
// resolved name rather than as something already on the request.
type AuthOnlyAuthenticator struct {
	logger     logger.Logger
	authProxy  string
	httpClient *http.Client
}

// newAuthOnlyAuthenticator returns the DLX's authentication hook, or nil when function-level
// authentication is disabled platform-wide. A nil hook is how the DLX skips the check entirely, which
// is what keeps the feature-flag-off path byte-identical to the behavior before this feature.
func (n *NuclioResourceScaler) newAuthOnlyAuthenticator() scalertypes.TargetAuthenticator {
	if !n.platformConfiguration.IsFunctionAuthenticationEnabled() {
		return nil
	}

	// the auth-proxy listens on the port it always listens on. The const is named for its function-pod
	// role of fronting the processor, which is not the job here - in the DLX pod authOnly forwards
	// nothing and only answers - but sharing it keeps this in step with the --routes the chart passes.
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

		// the same budget the auth-proxy gives its own auth-url call. Sharing it means a verdict that
		// lands in the last few milliseconds of the window may be missed and treated as a rejection;
		// an auth-url that slow is already failing the caller's request, so one timeout is enough
		Timeout: authproxy.AuthTimeout,

		// a browser-mode rejection is a 302 to the sign-in page: a verdict to hand back so the caller's
		// browser can follow it, not a hop for the DLX to take. This sentinel is not a failure - it tells
		// Do to return the 302 as the response. Left at the default, Do would instead try to fetch the
		// sign-in URL from inside the DLX pod, fail to reach it, and surface a redirect as a dead proxy.
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
}

// AuthenticateTarget implements scalertypes.TargetAuthenticator by delegating to the co-located
// auth-proxy. On false the rejection has already been written to res, so the caller must return
// without touching it.
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

	// the DLX resolved this name from the ingress, so it is what decides - set, not forwarded, so a
	// caller cannot name a function other than the one about to be scaled
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
