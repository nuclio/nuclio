/*
Copyright 2017 The Nuclio Authors.

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

package resource

import (
	"net/http"
	"strings"

	"github.com/nuclio/nuclio/pkg/dashboard"
	"github.com/nuclio/nuclio/pkg/dashboard/auth"
	"github.com/nuclio/nuclio/pkg/platform"
	"github.com/nuclio/nuclio/pkg/restful"

	"github.com/nuclio/errors"
	"github.com/nuclio/nuclio-sdk-go"
)

type resource struct {
	*restful.AbstractResource
}

func newResource(name string, resourceMethods []restful.ResourceMethod) *resource {
	return &resource{
		AbstractResource: restful.NewAbstractResource(name, resourceMethods),
	}
}

func (r *resource) getPlatform() platform.Platform {
	return r.getDashboard().Platform
}

func (r *resource) getNamespaceOrDefault(providedNamespace string) string {

	// if provided a namespace, use that
	if providedNamespace != "" {
		return providedNamespace
	}

	// get the default namespace we were created with
	return r.getDashboard().GetDefaultNamespace()
}

func (r *resource) getRequestAuthConfig(request *http.Request) (*platform.AuthConfig, error) {

	// TODO: move as a middleware for specific routes

	// if we're instructed to use the authorization header as an OIDC token
	if r.getDashboard().GetPlatformAuthorizationMode() == dashboard.PlatformAuthorizationModeAuthorizationHeaderOIDC {

		// make sure the Authorization header exists
		authorizationHeaderFromRequest := request.Header.Get("Authorization")
		if authorizationHeaderFromRequest == "" || !strings.HasPrefix(authorizationHeaderFromRequest, "Bearer ") {
			return nil, nuclio.WrapErrForbidden(errors.New("Missing Authorization header"))
		}

		// create the configuration
		return &platform.AuthConfig{
			Token: strings.TrimPrefix(authorizationHeaderFromRequest, "Bearer "),
		}, nil
	}

	// if we're instructed to use our service account for auth (or something invalid), just don't populate the auth config. this is
	// the default behavior
	return nil, nil
}

func (r *resource) getListenAddress() string {
	return r.getDashboard().ListenAddress
}

func (r *resource) headerValueIsTrue(request *http.Request, headerName string) bool {
	return strings.ToLower(request.Header.Get(headerName)) == "true"
}

func (r *resource) getDashboard() *dashboard.Server {
	return r.GetServer().(*dashboard.Server)
}

func (r *resource) addAuthMiddleware(options auth.Options) {
	authenticator := r.getDashboard().GetAuthenticator()
	r.Logger.DebugWith("Installing auth middleware on router",
		"authenticatorKind", authenticator.Kind(),
		"resourceName", r.GetName())
	r.GetRouter().Use(authenticator.Middleware(options))
}

func (r *resource) getCtxSession(request *http.Request) auth.Session {
	return request.Context().Value(auth.ContextKeyByKind(r.getDashboard().GetAuthenticator().Kind())).(auth.Session)
}
