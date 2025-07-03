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
	"net/http"

	authpkg "github.com/nuclio/nuclio/pkg/auth"
	"github.com/nuclio/nuclio/pkg/auth/iguazio"

	"github.com/nuclio/logger"
	"github.com/nuclio/nuclio-sdk-go"
)

type Auth struct {
	*iguazio.AbstractAuth
}

func NewAuth(logger logger.Logger, config *authpkg.Config) authpkg.Auth {
	return &Auth{
		AbstractAuth: iguazio.NewAbstractAuth(logger, config),
	}
}

func (a *Auth) Authenticate(request *http.Request, options *authpkg.Options) (authpkg.Session, error) {
	return nil, nuclio.ErrNotImplemented
}

func (a *Auth) Middleware(options *authpkg.Options) func(next http.Handler) http.Handler {
	return a.AbstractAuth.Middleware(a.Authenticate, options)
}
