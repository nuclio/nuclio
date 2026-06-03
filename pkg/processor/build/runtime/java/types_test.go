//go:build test_unit

/*
Copyright 2023 The Nuclio Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the Licensg.
You may obtain a copy of the License at

    http://www.apachg.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package java

import (
	"testing"

	"github.com/stretchr/testify/suite"
)

type testSuite struct {
	suite.Suite
}

func (suite *testSuite) TestSuccessfulParseDependencies() {

	dep, err := newDependency(`group: groupValue, name: nameValue, version: 0.2.1`)
	suite.Require().NoError(err)

	suite.Require().Equal("groupValue", dep.Group)
	suite.Require().Equal("nameValue", dep.Name)
	suite.Require().Equal("0.2.1", dep.Version)
}

// TestNewBuildAttributesDefaultsToMavenCentral verifies that an absent or empty
// repositories attribute falls back to mavenCentral(), preserving prior behaviour.
func (suite *testSuite) TestNewBuildAttributesDefaultsToMavenCentral() {
	for _, encoded := range []map[string]interface{}{
		nil,
		{},
		{"repositories": []string{}},
	} {
		buildAttributes, err := newBuildAttributes(encoded)
		suite.Require().NoError(err)
		suite.Require().Equal([]string{"mavenCentral()"}, buildAttributes.Repositories)
	}
}

// TestNewBuildAttributesRepositoryValidation covers the repository allowlist that
// guards against Groovy/Gradle code injection into the generated build.gradle
// (GHSA-3v79-m2cg-89ww).
func (suite *testSuite) TestNewBuildAttributesRepositoryValidation() {
	// the exact payload from the advisory: close the repositories block, run a
	// command, re-open the block
	injectionPayload := "mavenCentral()\n}\nprintln('[RCE-PROOF] ' + ['sh', '-c', 'id'].execute().text)\nrepositories {"

	for _, testCase := range []struct {
		name         string
		repositories []string
		expectError  bool
		expected     []string
	}{
		{
			name:         "built-in repositories accepted",
			repositories: []string{"mavenCentral()", "jcenter()", "google()", "mavenLocal()", "gradlePluginPortal()"},
			expected:     []string{"mavenCentral()", "jcenter()", "google()", "mavenLocal()", "gradlePluginPortal()"},
		},
		{
			name:         "surrounding whitespace trimmed",
			repositories: []string{"  mavenCentral()  "},
			expected:     []string{"mavenCentral()"},
		},
		{
			name:         "advisory injection payload rejected",
			repositories: []string{injectionPayload},
			expectError:  true,
		},
		{
			name:         "closing brace rejected",
			repositories: []string{"mavenCentral()}"},
			expectError:  true,
		},
		{
			name:         "newline rejected",
			repositories: []string{"mavenCentral()\nmavenLocal()"},
			expectError:  true,
		},
		{
			name:         "quote rejected",
			repositories: []string{"maven { url 'https://evil' }"},
			expectError:  true,
		},
		{
			name:         "empty string element rejected",
			repositories: []string{""},
			expectError:  true,
		},
		{
			name:         "whitespace-only element rejected",
			repositories: []string{"   "},
			expectError:  true,
		},
		{
			name:         "one invalid among valid rejects whole list",
			repositories: []string{"mavenCentral()", injectionPayload},
			expectError:  true,
		},
	} {
		suite.Run(testCase.name, func() {
			buildAttributes, err := newBuildAttributes(map[string]interface{}{
				"repositories": testCase.repositories,
			})

			if testCase.expectError {
				suite.Require().Error(err)
				return
			}

			suite.Require().NoError(err)
			suite.Require().Equal(testCase.expected, buildAttributes.Repositories)
		})
	}
}

func TestBuilderSuite(t *testing.T) {
	suite.Run(t, new(testSuite))
}
