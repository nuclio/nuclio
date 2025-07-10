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

	"github.com/golang-jwt/jwt/v5"
	"github.com/nuclio/errors"
)

const bearerPrefix = "Bearer "

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
	isJwtToken          bool
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

// TimeUntilExpiration parses the JWT access token from the Authorization header,
// extracts the 'exp' claim, and returns the time until expiration.
// If the remaining time is less than maxTime, it returns the actual remaining time.
// Otherwise, it returns maxTime. If token is invalid or expired, returns an error.
func (a *AuthParameters) TimeUntilExpiration(maxTime time.Duration) (time.Duration, error) {
	if !a.isJwtToken {
		return maxTime, nil
	}

	// Ensure the Authorization header is a Bearer token
	if len(a.authorizationHeader) <= len(bearerPrefix) || a.authorizationHeader[:len(bearerPrefix)] != bearerPrefix {
		return 0, errors.New("Authorization header is missing or not a Bearer token")
	}

	// Extract the JWT token string from the header
	tokenString := a.authorizationHeader[len(bearerPrefix):]

	// Parse the token without verifying the signature (used for claims inspection only)
	token, _, err := jwt.NewParser().ParseUnverified(tokenString, jwt.MapClaims{})
	if err != nil {
		return 0, errors.Wrap(err, "Failed to parse JWT")
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return 0, errors.New("Failed to parse claims from token")
	}

	// Extract the 'exp' claim (expiration time)
	expClaim, ok := claims["exp"]
	if !ok {
		return 0, errors.New("Missing `exp` field in token")
	}

	// Parse the 'exp' field, which is typically a float64 (seconds since epoch)
	expFloat, ok := expClaim.(float64)
	if !ok {
		return 0, errors.Errorf("Invalid `exp` claim type: %T", expClaim)
	}
	expUnix := int64(expFloat)

	expTime := time.Unix(expUnix, 0)
	remaining := time.Until(expTime)
	if remaining <= 0 {
		return 0, errors.New("Token is expired")
	}

	if remaining < maxTime {
		return remaining, nil
	}
	return maxTime, nil
}
