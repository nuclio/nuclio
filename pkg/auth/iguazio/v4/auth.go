/*
Copyright 2025 The Nuclio Authors.

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

package v4

import (
	"encoding/json"
	"fmt"
	"net/http"

	authpkg "github.com/nuclio/nuclio/pkg/auth"
	"github.com/nuclio/nuclio/pkg/auth/iguazio"
	"github.com/nuclio/nuclio/pkg/common"
	"github.com/nuclio/nuclio/pkg/common/headers"

	"github.com/nuclio/errors"
	"github.com/nuclio/logger"
	"github.com/nuclio/nuclio-sdk-go"
)

const groupType = "type.googleapis.com/group.Group"

type Auth struct {
	*iguazio.AbstractAuth
}

func NewAuth(logger logger.Logger, config *authpkg.Config) authpkg.Auth {
	authenticator := &Auth{}
	authenticator.AbstractAuth = iguazio.NewAbstractAuth(logger, config, authenticator)
	return authenticator
}

func (a *Auth) VerifySessionType(session interface{}) (authpkg.Session, bool) {
	if typedSession, ok := session.(*Session); ok {
		return typedSession, true
	}
	return nil, false
}

func (a *Auth) GetAuthParameters(request *http.Request, options *authpkg.Options) (*iguazio.AuthParameters, error) {
	ctx := request.Context()
	authorizationHeader := request.Header.Get(headers.AuthorizationHeader)
	oauth2Cookie, _ := request.Cookie(iguazio.OAuth2ProxyCookie)

	if oauth2Cookie == nil && authorizationHeader == "" {
		return nil, nuclio.NewErrForbidden("Authentication headers are missing")
	}

	authCookiesOnlyHeaderValue := common.CookiesToHeaderValue([]*http.Cookie{oauth2Cookie})

	// do not pass URL as it's always the same for v4 auth
	cacheKey := a.GenerateCacheKey(authCookiesOnlyHeaderValue, authorizationHeader, "")

	return iguazio.NewAuthParameters(
		ctx,
		authorizationHeader,
		authCookiesOnlyHeaderValue,
		a.GetConfig().Iguazio.VerificationURL,
		cacheKey), nil
}

func (a *Auth) ValidateResponse(response *http.Response) error {
	switch response.StatusCode {
	case http.StatusUnauthorized:
		return nuclio.NewErrUnauthorized("Invalid credentials")
	case http.StatusAccepted, http.StatusOK:
		return nil
	default:
		return nuclio.NewErrInternalServerError(fmt.Sprintf("Unexpected response from identity endpoint: %d", response.StatusCode))
	}
}

// BuildSessionFromResponse parses the response body and builds the session object
func (a *Auth) BuildSessionFromResponse(response *http.Response) (authpkg.Session, error) {
	var resp identityResponse
	if err := json.NewDecoder(response.Body).Decode(&resp); err != nil {
		return nil, errors.Wrap(err, "Failed to decode identity response")
	}

	var groupIDs []string
	for _, rel := range resp.Relationships {
		if rel.Type == groupType {
			groupIDs = append(groupIDs, rel.Metadata.ID)
		}
	}

	return NewSession(resp.Metadata.Username, groupIDs), nil
}
