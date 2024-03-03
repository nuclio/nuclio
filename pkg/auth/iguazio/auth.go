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

package iguazio

import (
	"bytes"
	"context"
	"crypto/sha256"
	"crypto/tls"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"time"

	authpkg "github.com/nuclio/nuclio/pkg/auth"
	"github.com/nuclio/nuclio/pkg/common"
	"github.com/nuclio/nuclio/pkg/common/headers"

	"github.com/nuclio/errors"
	"github.com/nuclio/logger"
	"github.com/nuclio/nuclio-sdk-go"
	"k8s.io/apimachinery/pkg/util/cache"
)

const (
	IguazioUsernameLabel                          string = "iguazio.com/username"
	IguazioDomainLabel                            string = "iguazio.com/domain"
	IguazioVerificationAndDataEnrichmentURLSuffix string = "_enrich_data"
)

type Auth struct {
	logger     logger.Logger
	config     *authpkg.Config
	httpClient *http.Client
	cache      *cache.LRUExpireCache
}

func NewAuth(logger logger.Logger, config *authpkg.Config) authpkg.Auth {
	return &Auth{
		logger: logger.GetChild("iguazio-auth"),
		config: config,
		httpClient: &http.Client{
			Timeout: config.Iguazio.Timeout,
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{InsecureSkipVerify: config.Iguazio.SkipTLSVerification},
			},
		},
		cache: cache.NewLRUExpireCache(config.Iguazio.CacheSize),
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

	url := a.config.Iguazio.VerificationURL
	if options.EnrichDataPlane {
		url = a.config.Iguazio.VerificationDataEnrichmentURL
	}

	method := a.config.Iguazio.VerificationMethod
	if method == "" {
		method = http.MethodPost
	}

	cacheKey := sha256.Sum256([]byte(cookie + authorization + url))

	// try resolve from cache
	if cacheData, found := a.cache.Get(cacheKey); found {
		return cacheData.(*authpkg.IguazioSession), nil
	}

	response, err := a.performHTTPRequest(request.Context(),
		method,
		url,
		nil,
		map[string]string{
			"authorization": authorization,
			"cookie":        cookie,
		})
	if err != nil {
		a.logger.WarnWithCtx(ctx,
			"Failed to perform http authentication request",
			"err", err.Error(),
		)
		return nil, errors.Wrap(err, "Failed to perform http POST request")
	}

	// auth failed
	if response.StatusCode == http.StatusUnauthorized {
		a.logger.WarnWithCtx(ctx,
			"Authentication failed",
			"authorizationHeaderLength", len(authHeaders["authorization"]),
			"cookieHeaderLength", len(authHeaders["cookie"]),
		)
		return nil, nuclio.NewErrUnauthorized("Authentication failed")
	}

	// not within range of 200
	if !(response.StatusCode >= http.StatusOK && response.StatusCode < 300) {
		a.logger.WarnWithCtx(ctx,
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
		a.logger.WarnWithCtx(ctx,
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

	a.cache.Add(cacheKey, authInfo, a.config.Iguazio.CacheExpirationTimeout)
	a.logger.InfoWithCtx(ctx,
		"Authentication succeeded",
		"url", url,
		"username", authInfo.GetUsername())
	return authInfo, nil
}

// Middleware will authenticate the incoming request and store the session within the request context
func (a *Auth) Middleware(options *authpkg.Options) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()
			session, err := a.Authenticate(r, options)
			if err != nil {
				a.logger.WarnWithCtx(ctx,
					"Authentication failed",
					"err", errors.GetErrorStackString(err, 10))
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
			a.logger.DebugWithCtx(ctx,
				"Successfully authenticated incoming request",
				"sessionUsername", session.GetUsername())
			enrichedCtx := context.WithValue(ctx, authpkg.IguazioContextKey, session)
			next.ServeHTTP(w, r.WithContext(enrichedCtx))
		})
	}
}

func (a *Auth) Kind() authpkg.Kind {
	return a.config.Kind
}

func (a *Auth) performHTTPRequest(ctx context.Context,
	method string,
	url string,
	body []byte,
	headers map[string]string) (*http.Response, error) {

	// create request
	request, err := http.NewRequestWithContext(ctx, method, url, bytes.NewBuffer(body))
	if err != nil {
		return nil, errors.Wrap(err, "Failed to create http request")
	}

	// attach headers
	for headerKey, headerValue := range headers {
		request.Header.Set(headerKey, headerValue)
	}

	var lastResponse *http.Response
	var lastError error
	if err := common.RetryUntilSuccessfulOnErrorPatterns(
		time.Second*60,
		time.Second*3,
		[]string{

			// usually when service is not up yet
			"EOF",
			"connection reset by peer",

			// tl;dr: we should actively retry on such errors, because Go won't as request might not be idempotent
			"server closed idle connection",
		},
		func(retryCounter int) (string, error) {

			// stop now if context is done
			if err := ctx.Err(); err != nil {
				return "", errors.Wrap(err, "Context is done")
			}

			if retryCounter > 0 {
				a.logger.WarnWithCtx(ctx,
					"Retrying authentication HTTP request",
					"retryCounter", retryCounter,
					"lastError", lastError)
			}

			// fire request
			lastResponse, err = a.httpClient.Do(request)
			if err != nil {
				lastError = err
				return err.Error(), errors.Wrap(err, "Failed to send HTTP request")
			}
			return "", nil
		}); err != nil {
		return lastResponse, errors.Wrap(err, "Failed to perform HTTP request")
	}

	return lastResponse, nil
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
