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
package nop

import (
	"context"
	"net/http"

	authpkg "github.com/nuclio/nuclio/pkg/auth"

	"github.com/nuclio/logger"
)

type Auth struct {
	logger     logger.Logger
	config     *authpkg.Config
	nopSession authpkg.Session
}

func NewAuth(logger logger.Logger, authConfig *authpkg.Config) authpkg.Auth {
	return &Auth{
		logger:     logger.GetChild("nop-auth"),
		config:     authConfig,
		nopSession: &authpkg.NopSession{},
	}
}

// Authenticate will do nothing
func (a *Auth) Authenticate(request *http.Request, options *authpkg.Options) (authpkg.Session, error) {
	return a.nopSession, nil
}

func (a *Auth) Middleware(options *authpkg.Options) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			session, _ := a.Authenticate(r, options)

			// nothing to do here
			ctx := context.WithValue(r.Context(), authpkg.NopContextKey, session)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func (a *Auth) Kind() authpkg.Kind {
	return a.config.Kind
}
