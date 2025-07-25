/*
Copyright 2023 The Nuclio Authors.

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

package v1

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"

	authpkg "github.com/nuclio/nuclio/pkg/auth"
	"github.com/nuclio/nuclio/pkg/auth/iguazio"
	"github.com/nuclio/nuclio/pkg/common"
	"github.com/nuclio/nuclio/pkg/common/headers"

	"github.com/nuclio/errors"
	"github.com/nuclio/logger"
	"github.com/nuclio/nuclio-sdk-go"
)

const SessionCookie = "session"

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
	authorization := request.Header.Get("authorization")
	cookie := request.Header.Get("cookie")

	if cookie == "" && authorization == "" {
		return nil, nuclio.NewErrForbidden("Authentication headers are missing")
	}

	url := a.resolveUrl(options)

	return iguazio.NewAuthParameters(
		ctx,
		authorization,
		cookie,
		url,
		false), nil
}

func (a *Auth) ValidateResponse(response *http.Response) error {
	// auth failed
	if response.StatusCode == http.StatusUnauthorized {
		return nuclio.NewErrUnauthorized("Authentication failed")
	}

	// not within range of 200
	if response.StatusCode < http.StatusOK || response.StatusCode >= 300 {
		return nuclio.NewErrUnauthorized("Authentication failed")
	}

	// auth succeeded
	return nil
}

func (a *Auth) resolveUrl(options *authpkg.Options) string {
	if options != nil && options.EnrichDataPlane {
		return a.GetConfig().Iguazio.VerificationDataEnrichmentURL
	}
	return a.GetConfig().Iguazio.VerificationURL
}

func (a *Auth) BuildSessionFromResponse(response *http.Response) (authpkg.Session, error) {
	encodedResponseBody, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to read response body")
	}

	responseBody := map[string]interface{}{}
	if err := json.Unmarshal(encodedResponseBody, &responseBody); err != nil {
		return nil, errors.Wrap(err, "Failed to unmarshal response body")
	}

	userID, groupIDs, err := a.resolveUserAndGroupIDsFromResponseBody(responseBody)
	if err != nil {
		a.Logger.WarnWith("Failed to resolve user and group IDs from response body, reading from headers",
			"err", err.Error())

		// for backwards compatibility
		userID = response.Header.Get(headers.UserID)
		if groupIDs == nil {
			groupIDs = response.Header.Values(headers.UserGroupIds)
		}
	}
	authInfo := NewSession(response.Header.Get(headers.RemoteUser),
		response.Header.Get(headers.V3IOSessionKey),
		userID, []string{})

	for _, groupID := range groupIDs {
		if groupID != "" {
			authInfo.GroupIDs = append(authInfo.GroupIDs, strings.Split(groupID, ",")...)
		}
	}

	return authInfo, nil
}

func (a *Auth) resolveUserAndGroupIDsFromResponseBody(responseBody map[string]interface{}) (string, []string, error) {

	attributes := []string{"data", "attributes", "context", "authentication"}
	authentication := common.GetAttributeRecursivelyFromMapStringInterface(responseBody, attributes)
	if authentication == nil {
		return "", nil, errors.New("Failed to find authentication in response body")
	}

	userId, ok := authentication["user_id"].(string)
	if !ok {
		return "", nil, errors.New("Failed to resolve user_id")
	}
	groupIds, ok := authentication["group_ids"].([]interface{})
	if !ok {
		return "", nil, errors.New("Failed to resolve group_ids")
	}

	var groupIdsStr []string
	for _, groupId := range groupIds {
		groupIdsStr = append(groupIdsStr, groupId.(string))
	}

	return userId, groupIdsStr, nil
}
