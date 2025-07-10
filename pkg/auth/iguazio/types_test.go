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
	"fmt"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/suite"
)

type AuthParametersTestSuite struct {
	suite.Suite
}

func (suite *AuthParametersTestSuite) TestTimeUntilExpiration() {
	tests := []struct {
		name           string
		authHeader     string
		isJwtToken     bool
		maxTime        time.Duration
		expectedResult time.Duration
		expectError    bool
	}{
		{
			name:           "ValidTokenWithShortExpiration",
			authHeader:     fmt.Sprintf("Bearer %s", suite.generateJWT(time.Now().Add(15*time.Second))),
			isJwtToken:     true,
			maxTime:        time.Minute,
			expectedResult: 15 * time.Second,
			expectError:    false,
		},
		{
			name:           "ValidTokenWithLongExpiration",
			authHeader:     fmt.Sprintf("Bearer %s", suite.generateJWT(time.Now().Add(2*time.Minute))),
			isJwtToken:     true,
			maxTime:        time.Minute,
			expectedResult: time.Minute,
			expectError:    false,
		},
		{
			name:           "ExpiredToken",
			authHeader:     fmt.Sprintf("Bearer %s", suite.generateJWT(time.Now().Add(-1*time.Minute))),
			isJwtToken:     true,
			maxTime:        time.Minute,
			expectedResult: 0,
			expectError:    true,
		},
		{
			name:           "NonJWTToken",
			authHeader:     "Bearer non-jwt-token",
			isJwtToken:     true,
			maxTime:        time.Minute,
			expectedResult: 0,
			expectError:    true,
		},
		{
			name:           "NonJWTMode",
			authHeader:     "Bearer irrelevant-token",
			isJwtToken:     false,
			maxTime:        time.Minute,
			expectedResult: time.Minute,
			expectError:    false,
		},
		{
			name:           "MissingBearerPrefix",
			authHeader:     "invalid-token",
			isJwtToken:     true,
			maxTime:        time.Minute,
			expectedResult: 0,
			expectError:    true,
		},
	}

	for _, testCase := range tests {
		suite.Run(testCase.name, func() {
			authParams := NewAuthParameters(
				context.Background(),
				testCase.authHeader,
				"",
				"http://somewhere.local",
				testCase.isJwtToken,
			)

			result, err := authParams.TimeUntilExpiration(testCase.maxTime)
			if testCase.expectError {
				suite.Require().Error(err)
			} else {
				suite.Require().NoError(err)

				// Allow a small tolerance for timing differences
				// because the actual expiration time may vary slightly as tests are running
				tolerance := 1 * time.Second
				suite.Require().InDelta(testCase.expectedResult.Seconds(), result.Seconds(), tolerance.Seconds(),
					"Expected result and actual result differ beyond tolerance")
			}
		})
	}
}

func (suite *AuthParametersTestSuite) generateJWT(expirationTime time.Time) string {
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"exp": expirationTime.Unix(),
	})
	tokenString, _ := token.SignedString([]byte("test-secret"))
	return tokenString
}

func TestAuthParametersTestSuite(t *testing.T) {
	suite.Run(t, new(AuthParametersTestSuite))
}
