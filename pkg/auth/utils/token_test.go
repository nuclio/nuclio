//go:build test_unit

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
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/suite"
)

const TestSecretKey = "test-secret-key"

type TokenUtilsTestSuite struct {
	suite.Suite
}

func (suite *TokenUtilsTestSuite) TestTimeUntilExpiration() {
	tests := []struct {
		name           string
		tokenString    string
		expectedResult time.Duration
		expectError    bool
	}{
		{
			name:           "ValidTokenWithShortExpiration",
			tokenString:    GenerateTestJWT(jwt.MapClaims{}, time.Now().Add(15*time.Second), TestSecretKey),
			expectedResult: 15 * time.Second,
			expectError:    false,
		},
		{
			name:           "ValidTokenWithLongExpiration",
			tokenString:    GenerateTestJWT(jwt.MapClaims{}, time.Now().Add(2*time.Minute), TestSecretKey),
			expectedResult: 2 * time.Minute,
			expectError:    false,
		},
		{
			name:           "ExpiredToken",
			tokenString:    GenerateTestJWT(jwt.MapClaims{}, time.Now().Add(-1*time.Minute), TestSecretKey),
			expectedResult: 0,
			expectError:    true,
		},
		{
			name:           "NonJWTToken",
			tokenString:    "non-jwt-token",
			expectedResult: 0,
			expectError:    true,
		},
	}

	for _, testCase := range tests {
		suite.Run(testCase.name, func() {
			result, err := TimeUntilExpiration(testCase.tokenString)
			if testCase.expectError {
				suite.Error(err)
			} else {
				suite.NoError(err)
				tolerance := 1 * time.Second
				suite.InDelta(testCase.expectedResult.Seconds(), result.Seconds(), tolerance.Seconds(),
					"Expected result and actual result differ beyond tolerance")
			}
		})
	}
}

func TestTokenUtilsTestSuite(t *testing.T) {
	suite.Run(t, new(TokenUtilsTestSuite))
}
