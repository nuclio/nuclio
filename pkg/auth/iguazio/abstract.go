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

package iguazio

import (
	"context"
	"crypto/sha256"
	"crypto/tls"
	"net/http"
	"strings"
	"time"

	authpkg "github.com/nuclio/nuclio/pkg/auth"
	"github.com/nuclio/nuclio/pkg/common"
	"github.com/nuclio/nuclio/pkg/common/headers"

	"github.com/nuclio/errors"
	"github.com/nuclio/logger"
	"k8s.io/apimachinery/pkg/util/cache"
)

const (
	IguazioUsernameLabel                          string = "iguazio.com/username"
	IguazioDomainLabel                            string = "iguazio.com/domain"
	IguazioVerificationAndDataEnrichmentURLSuffix string = "_enrich_data"
	OAuth2ProxyCookie                                    = "_oauth2_proxy"
)

type AbstractAuth struct {
	Logger     logger.Logger
	HttpClient *http.Client
	Cache      *cache.LRUExpireCache

	config        *authpkg.Config
	authenticator Authenticator
}

func NewAbstractAuth(logger logger.Logger, config *authpkg.Config, authenticator Authenticator) *AbstractAuth {
	return &AbstractAuth{
		Logger: logger.GetChild("iguazio-auth"),
		config: config,
		Cache:  cache.NewLRUExpireCache(config.Iguazio.CacheSize),
		HttpClient: &http.Client{
			Timeout: config.Iguazio.Timeout,
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{InsecureSkipVerify: config.Iguazio.SkipTLSVerification},
			},
		},
		authenticator: authenticator,
	}
}

func (a *AbstractAuth) Authenticate(request *http.Request, options *authpkg.Options) (authpkg.Session, error) {

	// Get authentication parameters from the request
	authParams, err := a.authenticator.GetAuthParameters(request, options)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to get authentication parameters")
	}

	// Attempt to retrieve the session from cache using the context as the key
	if cacheData := a.getFromCacheWithTypeCheck(authParams.cacheKey); cacheData != nil {
		return cacheData, nil
	}

	// Construct and send the identity request
	resp, err := a.ConstructAndSendIdentityRequest(authParams)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to construct and send identity request")
	}
	defer resp.Body.Close() //nolint: errcheck

	// Validate the status code of the response
	if err = a.authenticator.ValidateResponse(resp); err != nil {
		a.Logger.WarnWithCtx(request.Context(),
			"Authentication failed",
			"authorizationHeaderLength", len(authParams.authorizationHeader),
			"cookieHeaderLength", len(authParams.cookieHeader),
			"statusCode", resp.StatusCode,
			"err", err.Error())
		return nil, errors.Wrap(err, "Authentication failed")
	}

	// Extract the session from the response
	session, err := a.authenticator.BuildSessionFromResponse(resp)
	if err != nil {
		a.Logger.WarnWithCtx(request.Context(),
			"Failed to extract session from response",
			"err", err.Error())
		return nil, errors.Wrap(err, "Failed to extract session from response")
	}

	// Generate a cache key based on the authentication parameters
	a.Cache.Add(authParams.cacheKey, session, a.GetConfig().Iguazio.CacheExpirationTimeout)

	return session, nil
}

func (a *AbstractAuth) Middleware(options *authpkg.Options) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()
			session, err := a.Authenticate(r, options)
			if err != nil {
				a.Logger.WarnWithCtx(ctx,
					"Authentication failed",
					"err", errors.GetErrorStackString(err, 10))
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
			a.Logger.DebugWithCtx(ctx,
				"Successfully authenticated incoming request",
				"sessionUsername", session.GetUsername())
			enrichedCtx := context.WithValue(ctx, authpkg.IguazioContextKey, session)
			next.ServeHTTP(w, r.WithContext(enrichedCtx))
		})
	}
}

func (a *AbstractAuth) ConstructAndSendIdentityRequest(authParams *AuthParameters) (*http.Response, error) {
	req, err := a.buildIdentityRequest(authParams)
	if err != nil {
		return nil, err
	}

	resp, err := a.PerformHTTPRequest(authParams.ctx, req)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to perform request to identity service")
	}
	return resp, nil
}

func (a *AbstractAuth) GenerateCacheKey(authCookiesOnlyHeaderValue, authorizationHeader, url string) [32]byte {
	// generate cache key based on authorization header and URL
	return sha256.Sum256([]byte(authCookiesOnlyHeaderValue + authorizationHeader + url))
}

func (a *AbstractAuth) PerformHTTPRequest(ctx context.Context, request *http.Request) (*http.Response, error) {
	var lastResponse *http.Response
	var lastError error
	var err error

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
				a.Logger.WarnWithCtx(ctx,
					"Retrying authentication HTTP request",
					"retryCounter", retryCounter,
					"lastError", lastError)
			}

			// Send the HTTP request
			lastResponse, err = a.HttpClient.Do(request)
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

func (a *AbstractAuth) GetConfig() *authpkg.Config {
	return a.config
}

func (a *AbstractAuth) Kind() authpkg.Kind {
	return a.config.Kind
}

func (a *AbstractAuth) getFromCacheWithTypeCheck(cacheKey [32]byte) authpkg.Session {
	if cacheData, found := a.Cache.Get(cacheKey); found {
		if typedSession, ok := a.authenticator.VerifySessionType(cacheData); ok {
			return typedSession
		}
	}
	return nil
}

// buildIdentityRequest creates the HTTP request to the identity service
func (a *AbstractAuth) buildIdentityRequest(authParams *AuthParameters) (*http.Request, error) {
	method := a.GetConfig().Iguazio.VerificationMethod
	if method == "" {
		method = http.MethodGet
	}

	req, err := http.NewRequestWithContext(authParams.ctx, method, authParams.verificationURL, nil)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to create identity request")
	}

	if authParams.authorizationHeader != "" {
		req.Header.Set(headers.AuthorizationHeader, authParams.authorizationHeader)
	}

	if authParams.cookieHeader != "" {
		req.Header.Set(headers.CookieHeader, authParams.cookieHeader)
	}

	return req, nil
}

type AbstractSession struct {
	Username string
	GroupIDs []string
}

func (a *AbstractSession) GetUsername() string {
	return a.Username
}

func (a *AbstractSession) GetGroupIDs() []string {
	return a.GroupIDs
}

func (a *AbstractSession) CompileAuthorizationBasicHeader() string {
	return ""
}

func (a *AbstractSession) GetUserID() string {
	return ""
}

func (a *AbstractSession) GetPassword() string {
	return ""
}

func (a *AbstractSession) GetUserLabels() map[string]string {
	labels := make(map[string]string)
	fullUsername := a.GetUsername()
	// split email usernames to name and domain because '@' is an invalid character in kubernetes labels
	if strings.Contains(fullUsername, "@") {
		split := strings.Split(fullUsername, "@")
		labels[IguazioUsernameLabel] = split[0]
		labels[IguazioDomainLabel] = split[1]
	} else {
		labels[IguazioUsernameLabel] = fullUsername
	}
	return labels
}
