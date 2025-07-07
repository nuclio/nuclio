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
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	authpkg "github.com/nuclio/nuclio/pkg/auth"
	"github.com/nuclio/nuclio/pkg/auth/iguazio"
	"github.com/nuclio/nuclio/pkg/common/headers"

	"github.com/nuclio/errors"
	"github.com/nuclio/logger"
	"github.com/nuclio/nuclio-sdk-go"
)

const OAuth2ProxyCookie = "_oauth2_proxy"
const groupType = "type.googleapis.com/group.Group"

type Auth struct {
	*iguazio.AbstractAuth
}

func NewAuth(logger logger.Logger, config *authpkg.Config) (authpkg.Auth, error) {
	return &Auth{
		AbstractAuth: iguazio.NewAbstractAuth(logger, config),
	}, nil
}

func (a *Auth) Authenticate(request *http.Request, options *authpkg.Options) (authpkg.Session, error) {
	ctx := request.Context()
	authorizationHeader := request.Header.Get(headers.AuthorizationHeader)
	cookie, _ := request.Cookie(OAuth2ProxyCookie)

	if cookie == nil && authorizationHeader == "" {
		return nil, nuclio.NewErrForbidden("Authentication headers are missing")
	}

	resp, err := a.constructAndSendIdentityRequest(ctx, authorizationHeader, cookie)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to construct and send identity request")
	}
	defer resp.Body.Close() //nolint: errcheck

	switch resp.StatusCode {
	case http.StatusUnauthorized:
		return nil, nuclio.NewErrUnauthorized("Invalid credentials")
	case http.StatusAccepted:
		return a.buildSessionFromResponse(resp.Body)
	default:
		return nil, nuclio.NewErrInternalServerError(fmt.Sprintf("Unexpected response from identity endpoint: %d", resp.StatusCode))
	}
}

func (a *Auth) Middleware(options *authpkg.Options) func(next http.Handler) http.Handler {
	return a.AbstractAuth.Middleware(a.Authenticate, options)
}

func (a *Auth) constructAndSendIdentityRequest(ctx context.Context, authHeader string, cookie *http.Cookie) (*http.Response, error) {
	req, err := a.buildIdentityRequest(ctx, authHeader, cookie)
	if err != nil {
		return nil, err
	}

	resp, err := a.PerformHTTPRequest(ctx, req)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to perform request to identity service")
	}
	return resp, nil
}

// buildIdentityRequest creates the HTTP request to the identity service
func (a *Auth) buildIdentityRequest(ctx context.Context, authHeader string, cookie *http.Cookie) (*http.Request, error) {
	method := a.GetConfig().Iguazio.VerificationMethod
	if method == "" {
		method = http.MethodPost
	}

	req, err := http.NewRequestWithContext(ctx, method, a.GetConfig().Iguazio.VerificationURL, nil)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to create identity request")
	}

	if authHeader != "" {
		req.Header.Set(headers.AuthorizationHeader, authHeader)
	}

	if cookie != nil {
		req.Header.Set("Cookie", fmt.Sprintf("%s=%s", cookie.Name, cookie.Value))
	}

	return req, nil
}

// buildSessionFromResponse parses the response body and builds the session object
func (a *Auth) buildSessionFromResponse(body io.Reader) (authpkg.Session, error) {
	var resp identityResponse
	if err := json.NewDecoder(body).Decode(&resp); err != nil {
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
