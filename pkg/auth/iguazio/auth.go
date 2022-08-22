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
package iguazio

import (
	"bytes"
	"context"
	"crypto/sha256"
	"crypto/tls"
	"net/http"
	"strings"

	authpkg "github.com/nuclio/nuclio/pkg/auth"

	"github.com/nuclio/errors"
	"github.com/nuclio/logger"
	"github.com/nuclio/nuclio-sdk-go"
	"k8s.io/apimachinery/pkg/util/cache"
)

const (
	IguzioUsernameLabel                          string = "iguazio.com/username"
	IguzioVerificationAndDataEnrichmentURLSuffix string = "_enrich_data"
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
				TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
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

	cacheKey := sha256.Sum256([]byte(cookie + authorization + url))

	// try resolve from cache
	if cacheData, found := a.cache.Get(cacheKey); found {
		return cacheData.(*authpkg.IguazioSession), nil
	}

	response, err := a.performHTTPRequest(request.Context(),
		http.MethodPost,
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

	authInfo := &authpkg.IguazioSession{
		Username:   response.Header.Get("x-remote-user"),
		SessionKey: response.Header.Get("x-v3io-session-key"),
		UserID:     response.Header.Get("x-user-id"),
	}

	for _, groupID := range response.Header.Values("x-user-group-ids") {
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
	req, err := http.NewRequestWithContext(ctx, method, url, bytes.NewBuffer(body))
	if err != nil {
		return nil, errors.Wrap(err, "Failed to create http request")
	}

	// attach headers
	for headerKey, headerValue := range headers {
		req.Header.Set(headerKey, headerValue)
	}

	// fire request
	resp, err := a.httpClient.Do(req)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to send HTTP request")
	}

	return resp, nil
}
