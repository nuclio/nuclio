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
	"context"
	"net/http"

	"github.com/nuclio/nuclio/pkg/auth"
	"github.com/nuclio/nuclio/pkg/common"
	"github.com/nuclio/nuclio/pkg/common/headers"
	"github.com/nuclio/nuclio/pkg/functionconfig"
	kubeclient "github.com/nuclio/nuclio/pkg/platform/kube/clients/kube"
	nuclioioclient "github.com/nuclio/nuclio/pkg/platform/kube/clients/nuclio/clientset/versioned"
	"github.com/nuclio/nuclio/pkg/platform/kube/ingress"

	"github.com/mitchellh/mapstructure"
	"github.com/nuclio/errors"
	"github.com/nuclio/logger"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// authOnlyAuthenticator serves the DLX (authOnly) topology: a single sidecar guards every scaled-to-zero
// function, so it resolves each request's auth config from the target function's CRD (fresh, no cache).
// Bind it to a specific request via bindRequest to get a TargetAuthenticator the DLX can call by
// function name alone.
type authOnlyAuthenticator struct {
	*abstractAuthenticator
	nuclioClientSet nuclioioclient.Interface
	scrubber        *functionconfig.Scrubber
	namespace       string
}

// boundAuthenticator pairs an authOnlyAuthenticator with a specific request/response so that
// AuthenticateTarget can be called with only a function name.
type boundAuthenticator struct {
	*authOnlyAuthenticator
	responseWriter http.ResponseWriter
	request        *http.Request
}

// AuthenticateTarget sets the target-function header and delegates to Authenticate, so the DLX needs
// only the function name — the same code path as an inbound HTTP request to the /auth endpoint.
func (b *boundAuthenticator) AuthenticateTarget(functionName string) bool {
	b.request.Header.Set(headers.TargetFunctionName, functionName)
	return b.Authenticate(b.responseWriter, b.request)
}

// NewAuthOnlyAuthenticator creates an authenticator that resolves auth config per request from the function
// CRD. It is used both by the sidecar /auth endpoint (via Authenticate) and, bound to a request, by the
// in-process DLX (via AuthenticateTarget). The kube client is used to restore scrubbed credentials
// (e.g. the basicAuth password, stored as a $ref: to the function's dedicated Secret).
func NewAuthOnlyAuthenticator(parentLogger logger.Logger,
	authURL string,
	signinURL string,
	authKind auth.Kind,
	nuclioClientSet nuclioioclient.Interface,
	kubeClientSet kubeclient.Client,
	namespace string) (Authenticator, error) {
	parentLogger.InfoWith("Creating auth-only authenticator", "authURL", authURL, "signinURL", signinURL, "namespace", namespace)
	abstract, err := newAbstractAuthenticator(parentLogger, authURL, signinURL, authKind)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to create abstract authenticator")
	}
	return &authOnlyAuthenticator{
		abstractAuthenticator: abstract,
		nuclioClientSet:       nuclioClientSet,

		// sensitive-field regexes are only needed for scrubbing; restoring just replaces $ref:
		// placeholders from the function's Secret, so nil is fine here
		scrubber:  functionconfig.NewScrubber(parentLogger, nil, kubeClientSet),
		namespace: namespace,
	}, nil
}

// Authenticate resolves the target function from the request header, then authenticates against it.
func (a *authOnlyAuthenticator) Authenticate(responseWriter http.ResponseWriter, request *http.Request) bool {
	functionName := request.Header.Get(headers.TargetFunctionName)
	if functionName == "" {

		// can't determine the target -> fail closed
		a.logger.Warn("Target function name header is missing, failing closed")
		http.Error(responseWriter, "Forbidden", http.StatusForbidden)
		return false
	}
	return a.authenticateTarget(responseWriter, request, functionName)
}

// bindRequest returns a TargetAuthenticator bound to the given request/response, so AuthenticateTarget
// can be called with only a function name.
func (a *authOnlyAuthenticator) bindRequest(responseWriter http.ResponseWriter, request *http.Request) TargetAuthenticator {
	return &boundAuthenticator{
		authOnlyAuthenticator: a,
		responseWriter:        responseWriter,
		request:               request,
	}
}

// authenticateTarget authenticates the request against the named target function, resolving its
// auth config from the function CRD.
func (a *authOnlyAuthenticator) authenticateTarget(responseWriter http.ResponseWriter, request *http.Request, functionName string) bool {
	authConfig, err := a.getAuthSpec(request.Context(), functionName)
	if err != nil {

		// can't determine the mode -> fail closed
		a.logger.WarnWithCtx(request.Context(),
			"Failed to resolve function auth config, failing closed",
			"functionName", functionName,
			"err", err.Error())
		http.Error(responseWriter, "Forbidden", http.StatusForbidden)
		return false
	}
	return a.decide(responseWriter, request, authConfig)
}

// getAuthSpec reads the target function's CRD, extracts its auth config, and (only for basicAuth)
// restores scrubbed credentials from the function's dedicated Secret.
func (a *authOnlyAuthenticator) getAuthSpec(ctx context.Context, functionName string) (FunctionAuthConfig, error) {
	function, err := a.nuclioClientSet.
		NuclioV1beta1().
		NuclioFunctions(a.namespace).
		Get(ctx, functionName, metav1.GetOptions{})
	if err != nil {
		return FunctionAuthConfig{}, errors.Wrapf(err, "Failed to get function %s", functionName)
	}

	authConfig, err := functionAuthConfigFromSpec(&function.Spec)
	if err != nil {
		return FunctionAuthConfig{}, errors.Wrapf(err, "Failed to read auth config for function %s", functionName)
	}

	// the scrubber is only needed for basicAuth: it replaces $ref: placeholders with the real
	// credentials stored in the function's dedicated Secret; other modes have no sensitive fields
	if authConfig.Mode != auth.AuthenticationModeBasicAuth {
		return authConfig, nil
	}

	functionConfig := &functionconfig.Config{
		Meta: functionconfig.Meta{
			Name:      function.Name,
			Namespace: function.Namespace,
		},
		Spec: function.Spec,
	}
	restoredConfig, err := a.scrubber.RestoreFunctionConfig(ctx, functionConfig, common.KubePlatformName)
	if err != nil {
		return FunctionAuthConfig{}, errors.Wrapf(err, "Failed to restore function %s secrets", functionName)
	}

	return functionAuthConfigFromSpec(&restoredConfig.Spec)
}

// functionAuthConfigFromSpec resolves the authentication config from the function's HTTP trigger.
func functionAuthConfigFromSpec(spec *functionconfig.Spec) (FunctionAuthConfig, error) {
	for _, httpTrigger := range functionconfig.GetTriggersByKind(spec.Triggers, "http") {
		return functionAuthConfigFromAttributes(httpTrigger.Attributes)
	}

	// no HTTP trigger -> nothing to authenticate
	return FunctionAuthConfig{Mode: auth.AuthenticationModeNone}, nil
}

// functionAuthConfigFromAttributes decodes authenticationMode + authentication.basicAuth from an HTTP
// trigger's free-form attributes (HLD §2.1.2), reusing the shared ingress.Authentication spec for the credentials.
func functionAuthConfigFromAttributes(attributes map[string]interface{}) (FunctionAuthConfig, error) {
	decoded := struct {
		AuthenticationMode string                  `mapstructure:"authenticationMode"`
		Authentication     *ingress.Authentication `mapstructure:"authentication"`
	}{}
	if err := mapstructure.Decode(attributes, &decoded); err != nil {
		return FunctionAuthConfig{}, errors.Wrap(err, "Failed to decode HTTP trigger attributes")
	}

	mode := auth.AuthenticationMode(decoded.AuthenticationMode)
	if mode == "" {
		mode = auth.AuthenticationModeNone
	}

	authConfig := FunctionAuthConfig{Mode: mode}
	if decoded.Authentication != nil && decoded.Authentication.BasicAuth != nil {
		authConfig.BasicAuthUsername = decoded.Authentication.BasicAuth.Username
		authConfig.BasicAuthPassword = decoded.Authentication.BasicAuth.Password
	}

	if mode == auth.AuthenticationModeBasicAuth {
		if authConfig.BasicAuthUsername == "" {
			return FunctionAuthConfig{}, errors.New("Basic-auth username must be provided")
		}
		if authConfig.BasicAuthPassword == "" {
			return FunctionAuthConfig{}, errors.New("Basic-auth password must be provided")
		}
	}

	return authConfig, nil
}
