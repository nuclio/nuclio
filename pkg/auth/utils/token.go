/*
Copyright 2026 The Nuclio Authors.

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

package utils

import (
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/nuclio/errors"
)

const BearerPrefix = "Bearer "

// TimeUntilExpiration parses the JWT access token from the Authorization header,
// extracts the 'exp' claim, and returns the time until expiration.
func TimeUntilExpiration(tokenString string) (time.Duration, error) {

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

	return remaining, nil
}

// GenerateTestJWT generates a JWT token string for testing purposes with the given claims and expiration time.
func GenerateTestJWT(claims jwt.MapClaims, expirationTime time.Time, secret string) string {
	claims["exp"] = expirationTime.Unix()
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, _ := token.SignedString([]byte(secret))
	return tokenString
}
