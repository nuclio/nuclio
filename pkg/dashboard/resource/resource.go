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
	"fmt"
	"net/http"
	"strings"

	"github.com/nuclio/nuclio/pkg/dashboard"
	"github.com/nuclio/nuclio/pkg/dashboard/opa"
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
	return r.GetServer().(*dashboard.Server).Platform
}

func (r *resource) getNamespaceOrDefault(providedNamespace string) string {

	// if provided a namespace, use that
	if providedNamespace != "" {
		return providedNamespace
	}

	// get the default namespace we were created with
	return r.GetServer().(*dashboard.Server).GetDefaultNamespace()
}

func (r *resource) getRequestAuthConfig(request *http.Request) (*platform.AuthConfig, error) {

	// if we're instructed to use the authorization header as an OIDC token
	if r.GetServer().(*dashboard.Server).GetPlatformAuthorizationMode() == dashboard.PlatformAuthorizationModeAuthorizationHeaderOIDC {

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
	return r.GetServer().(*dashboard.Server).ListenAddress
}

func (r *resource) headerValueIsTrue(request *http.Request, headerName string) bool {
	return strings.ToLower(request.Header.Get(headerName)) == "true"
}

func (r *resource) getUserAndGroupIdsFromHeaders(request *http.Request) []string {
	var ids []string

	userID := request.Header.Get(opa.UserIDHeader)
	userGroupIdsStr := request.Header.Get(opa.UserGroupIdsHeader)

	if userID != "" {
		ids = append(ids, userID)
	}

	if userGroupIdsStr != "" {
		ids = append(ids, strings.Split(userGroupIdsStr, ",")...)
	}

	return ids
}

func (r *resource) queryOPAPermissions(request *http.Request,
	resource string,
	action opa.Action,
	raiseForbidden bool) (bool, error) {
	ids := r.getUserAndGroupIdsFromHeaders(request)
	allowed, err := r.GetServer().(*dashboard.Server).OPAClient.QueryPermissions(resource, action, ids)
	if err != nil {
		return allowed, errors.Wrapf(err, "Failed to check %s permissions for resource %s", action, resource)
	}
	if !allowed && raiseForbidden {
		return false, nuclio.NewErrForbidden(fmt.Sprintf("Not allowed to %s resource %s", action, resource))
	}
	return allowed, nil
}
