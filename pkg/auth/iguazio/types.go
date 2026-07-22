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
	"net/http"
	"time"

	authpkg "github.com/nuclio/nuclio/pkg/auth"
	"github.com/nuclio/nuclio/pkg/auth/utils"

	"github.com/nuclio/errors"
)

type Authenticator interface {
	// GetAuthParameters retrieves the authentication parameters from the request
	GetAuthParameters(request *http.Request, options *authpkg.Options) (*AuthParameters, error)

	// ValidateResponse checks the response from the Iguazio session verification endpoint
	ValidateResponse(response *http.Response) error

	// BuildSessionFromResponse constructs a session from the response received from the Iguazio session verification endpoint
	BuildSessionFromResponse(response *http.Response, authParams *AuthParameters) (authpkg.Session, error)

	// VerifySessionType checks if the provided session is of the expected type
	// and returns (typedValue, true) if valid and (nil, false) otherwise
	VerifySessionType(session interface{}) (authpkg.Session, bool)
}

type AuthParameters struct {
	ctx                 context.Context
	authorizationHeader string
	cookieHeader        string
	verificationURL     string
	isJwtToken          bool

	// forwarded to the verification endpoint so it can select the right
	// validator (e.g. "sa" for a service-account token). Optional.
	authenticatorKind string
}

func NewAuthParameters(ctx context.Context, authorizationHeader, cookieHeader, verificationURL string, isJwtToken bool) *AuthParameters {
	return &AuthParameters{
		ctx:                 ctx,
		authorizationHeader: authorizationHeader,
		cookieHeader:        cookieHeader,
		verificationURL:     verificationURL,
		isJwtToken:          isJwtToken,
	}
}

func (a *AuthParameters) GenerateCacheKey() ([32]byte, error) {
	if a.authorizationHeader == "" {
		return [32]byte{}, errors.New("Authorization header is empty")
	}
	// generate cache key based on authorization header and URL
	return sha256.Sum256([]byte(a.authorizationHeader + a.verificationURL)), nil
}

func (a *AuthParameters) GetAuthorizationHeader() string {
	return a.authorizationHeader
}

// SetAuthenticatorKind sets the X-IGZ-Authenticator-Kind value to forward to the verification endpoint.
func (a *AuthParameters) SetAuthenticatorKind(authenticatorKind string) {
	if authenticatorKind != "" {
		a.authenticatorKind = authenticatorKind
	}
}

// TimeUntilExpiration parses the JWT access token from the Authorization header,
// extracts the 'exp' claim, and returns the time until expiration.
// If the remaining time is less than maxTime, it returns the actual remaining time.
// Otherwise, it returns maxTime. If token is invalid or expired, returns an error.
func (a *AuthParameters) TimeUntilExpiration(maxTime time.Duration) (time.Duration, error) {
	if !a.isJwtToken {
		return maxTime, nil
	}

	// Ensure the Authorization header is a Bearer token
	if len(a.authorizationHeader) <= len(utils.BearerPrefix) || a.authorizationHeader[:len(utils.BearerPrefix)] != utils.BearerPrefix {
		return 0, errors.New("Authorization header is missing or not a Bearer token")
	}

	// Extract the JWT token string from the header
	tokenString := a.authorizationHeader[len(utils.BearerPrefix):]

	remaining, err := utils.TimeUntilExpiration(tokenString)
	if err != nil {
		return 0, errors.Wrap(err, "Failed to get time until expiration from token")
	}

	if remaining < maxTime {
		return remaining, nil
	}
	return maxTime, nil
}
