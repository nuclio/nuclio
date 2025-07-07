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
	"crypto/sha256"
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
	"k8s.io/apimachinery/pkg/util/cache"
)

type Auth struct {
	*iguazio.AbstractAuth
	cache *cache.LRUExpireCache
}

func NewAuth(logger logger.Logger, config *authpkg.Config) authpkg.Auth {
	return &Auth{
		AbstractAuth: iguazio.NewAbstractAuth(logger, config),
		cache:        cache.NewLRUExpireCache(config.Iguazio.CacheSize),
	}
}

// Authenticate will ask IguazioConfig session verification endpoint to verify the request session
// and enrich with session metadata
func (a *Auth) Authenticate(request *http.Request, options *authpkg.Options) (authpkg.Session, error) {
	ctx := request.Context()
	authorization := request.Header.Get("authorization")
	cookie := request.Header.Get("cookie")

	if options == nil {
		options = &authpkg.Options{}
	}

	if cookie == "" && authorization == "" {
		return nil, nuclio.NewErrForbidden("Authentication headers are missing")
	}

	authHeaders := map[string]string{
		"authorization": authorization,
		"cookie":        cookie,
	}

	url := a.GetConfig().Iguazio.VerificationURL
	if options.EnrichDataPlane {
		url = a.GetConfig().Iguazio.VerificationDataEnrichmentURL
	}

	method := a.GetConfig().Iguazio.VerificationMethod
	if method == "" {
		method = http.MethodPost
	}

	cacheKey := sha256.Sum256([]byte(cookie + authorization + url))

	// try resolve from cache
	if cacheData, found := a.cache.Get(cacheKey); found {
		return cacheData.(*authpkg.IguazioSession), nil
	}

	response, err := a.PerformHTTPRequest(request.Context(),
		method,
		url,
		nil,
		map[string]string{
			"authorization": authorization,
			"cookie":        cookie,
		})
	if err != nil {
		a.Logger.WarnWithCtx(ctx,
			"Failed to perform http authentication request",
			"err", err.Error(),
		)
		return nil, errors.Wrap(err, "Failed to perform http POST request")
	}

	// auth failed
	if response.StatusCode == http.StatusUnauthorized {
		a.Logger.WarnWithCtx(ctx,
			"Authentication failed",
			"authorizationHeaderLength", len(authHeaders["authorization"]),
			"cookieHeaderLength", len(authHeaders["cookie"]),
		)
		return nil, nuclio.NewErrUnauthorized("Authentication failed")
	}

	// not within range of 200
	if !(response.StatusCode >= http.StatusOK && response.StatusCode < 300) {
		a.Logger.WarnWithCtx(ctx,
			"Unexpected authentication status code",
			"authorizationHeaderLength", len(authHeaders["authorization"]),
			"cookieHeaderLength", len(authHeaders["cookie"]),
			"statusCode", response.StatusCode,
		)
		return nil, nuclio.NewErrUnauthorized("Authentication failed")
	}

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
		a.Logger.WarnWithCtx(ctx,
			"Failed to resolve user and group IDs from response body, reading from headers",
			"err", err.Error())

		// for backwards compatibility
		userID = response.Header.Get(headers.UserID)
		if groupIDs == nil {
			groupIDs = response.Header.Values(headers.UserGroupIds)
		}
	}

	authInfo := &authpkg.IguazioSession{
		Username:   response.Header.Get(headers.RemoteUser),
		SessionKey: response.Header.Get(headers.V3IOSessionKey),
		UserID:     userID,
	}

	for _, groupID := range groupIDs {
		if groupID != "" {
			authInfo.GroupIDs = append(authInfo.GroupIDs, strings.Split(groupID, ",")...)
		}
	}

	a.cache.Add(cacheKey, authInfo, a.GetConfig().Iguazio.CacheExpirationTimeout)
	a.Logger.InfoWithCtx(ctx,
		"Authentication succeeded",
		"url", url,
		"username", authInfo.GetUsername())
	return authInfo, nil
}

// Middleware will authenticate the incoming request and store the session within the request context
func (a *Auth) Middleware(options *authpkg.Options) func(next http.Handler) http.Handler {
	return a.AbstractAuth.Middleware(a.Authenticate, options)
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
