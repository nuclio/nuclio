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
	"net/http"

	authpkg "github.com/nuclio/nuclio/pkg/auth"
)

type Authenticator interface {
	// GetAuthParameters retrieves the authentication parameters from the request
	GetAuthParameters(request *http.Request, options *authpkg.Options) (*AuthParameters, error)

	// ValidateResponse checks the response from the Iguazio session verification endpoint
	ValidateResponse(response *http.Response) error

	// BuildSessionFromResponse constructs a session from the response received from the Iguazio session verification endpoint
	BuildSessionFromResponse(response *http.Response) (authpkg.Session, error)

	// VerifySessionType checks if the provided session is of the expected type
	// and returns (typedValue, true) if valid and (nil, false) otherwise
	VerifySessionType(session interface{}) (authpkg.Session, bool)
}

type AuthParameters struct {
	ctx                 context.Context
	authorizationHeader string
	cookieHeader        string
	verificationURL     string
	cacheKey            [32]byte
}

func NewAuthParameters(ctx context.Context, authorizationHeader, cookieHeader, verificationURL string, cacheKey [32]byte) *AuthParameters {
	return &AuthParameters{
		ctx:                 ctx,
		authorizationHeader: authorizationHeader,
		cookieHeader:        cookieHeader,
		verificationURL:     verificationURL,
		cacheKey:            cacheKey,
	}
}
