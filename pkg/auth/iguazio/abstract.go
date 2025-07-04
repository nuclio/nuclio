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
	"crypto/tls"
	"net/http"
	"time"

	authpkg "github.com/nuclio/nuclio/pkg/auth"
	"github.com/nuclio/nuclio/pkg/common"

	"github.com/nuclio/errors"
	"github.com/nuclio/logger"
	"k8s.io/apimachinery/pkg/util/cache"
)

const (
	IguazioUsernameLabel                          string = "iguazio.com/username"
	IguazioDomainLabel                            string = "iguazio.com/domain"
	IguazioVerificationAndDataEnrichmentURLSuffix string = "_enrich_data"
)

type AbstractAuth struct {
	Logger     logger.Logger
	HttpClient *http.Client
	Cache      *cache.LRUExpireCache

	config *authpkg.Config
}

func NewAbstractAuth(logger logger.Logger, config *authpkg.Config) *AbstractAuth {
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
	}
}

func (a *AbstractAuth) Middleware(authenticateFunc func(*http.Request, *authpkg.Options) (authpkg.Session, error), options *authpkg.Options) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()
			session, err := authenticateFunc(r, options)
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

func (a *AbstractSession) CompileAuthorizationBasic() string {
	return ""
}

func (a *AbstractSession) GetUserID() string {
	return ""
}

func (a *AbstractSession) GetPassword() string {
	return ""
}

func (a *AbstractSession) GetUserLabels() map[string]string {
	return nil
}
