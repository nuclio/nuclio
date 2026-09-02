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
	"bytes"
	"testing"
	"text/template"

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
		// the following payloads execute code inside the repositories {} block without
		// any block-breakout metacharacters (no brace, newline or quote); the structural
		// "name()" rule must still reject them (GHSA-3v79-m2cg-89ww)
		{
			name:         "method call with argument rejected",
			repositories: []string{"System.exit(0)"},
			expectError:  true,
		},
		{
			name:         "runtime exec rejected",
			repositories: []string{"Runtime.getRuntime().exec(/usr/bin/id/)"},
			expectError:  true,
		},
		{
			name:         "slashy-string execute rejected",
			repositories: []string{"/id/.execute()"},
			expectError:  true,
		},
		{
			name:         "char-cast string construction rejected",
			repositories: []string{"((char)105).toString().concat(((char)100).toString()).execute()"},
			expectError:  true,
		},
		{
			name:         "method chain rejected",
			repositories: []string{"mavenCentral().toString()"},
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

// TestNewDependencyRejectsGroovyInjection verifies newDependency rejects Group/Name/Version
// values that would break out of the single-quoted Groovy literal they're rendered into.
func (suite *testSuite) TestNewDependencyRejectsGroovyInjection() {
	// closes the group string literal, runs a command, then reopens a literal so the
	// generated line still parses as valid Groovy
	injectionPayload := `x'; new ProcessBuilder(['sh','-c','id']).start(); y='`

	for _, testCase := range []struct {
		name           string
		raw            string
		expectError    bool
		expectedResult *dependency
	}{
		{
			name: "well-formed coordinate accepted",
			raw:  `group: com.example, name: my-lib, version: 1.2.3`,
			expectedResult: &dependency{
				Group:   "com.example",
				Name:    "my-lib",
				Version: "1.2.3",
			},
		},
		{
			name:        "single quote breakout in group rejected",
			raw:         `group: "` + injectionPayload + `", name: n, version: v`,
			expectError: true,
		},
		{
			name:        "single quote breakout in name rejected",
			raw:         `group: g, name: "` + injectionPayload + `", version: v`,
			expectError: true,
		},
		{
			name:        "single quote breakout in version rejected",
			raw:         `group: g, name: n, version: "` + injectionPayload + `"`,
			expectError: true,
		},
	} {
		suite.Run(testCase.name, func() {
			dep, err := newDependency(testCase.raw)

			if testCase.expectError {
				suite.Require().Error(err, "expected the malicious dependency %q to be rejected", testCase.raw)
				return
			}

			suite.Require().NoError(err)
			suite.Require().NotNil(dep)
			suite.Require().Equal(testCase.expectedResult.Group, dep.Group)
			suite.Require().Equal(testCase.expectedResult.Name, dep.Name)
			suite.Require().Equal(testCase.expectedResult.Version, dep.Version)
		})
	}
}

// TestGradleBuildScriptDependencyInjection verifies a malicious dependency is rejected before
// it reaches the build.gradle template, while a legitimate dependency still renders normally.
func (suite *testSuite) TestGradleBuildScriptDependencyInjection() {
	for _, testCase := range []struct {
		name           string
		rawDependency  string
		expectError    bool
		expectedRender string
	}{
		{
			name:          "malicious dependency rejected before reaching the template",
			rawDependency: `group: "x'; new ProcessBuilder(['sh','-c','id']).start(); y='", name: n, version: v`,
			expectError:   true,
		},
		{
			name:           "legitimate dependency renders through the template",
			rawDependency:  "group: com.google.code.gson, name: gson, version: 2.8.9",
			expectedRender: "implementation group: 'com.google.code.gson', name: 'gson', version: '2.8.9'",
		},
	} {
		suite.Run(testCase.name, func() {
			dependencies, err := (&java{}).parseDependencies([]string{testCase.rawDependency})

			if testCase.expectError {
				suite.Require().Error(err)
				return
			}
			suite.Require().NoError(err)

			buildAttributes, err := newBuildAttributes(nil)
			suite.Require().NoError(err)

			tmpl, err := template.New("gradleBuildScript").Parse((&java{}).getGradleBuildScriptTemplateContents())
			suite.Require().NoError(err)

			var rendered bytes.Buffer
			err = tmpl.Execute(&rendered, map[string]interface{}{
				"Dependencies": dependencies,
				"Repositories": buildAttributes.Repositories,
			})
			suite.Require().NoError(err)
			suite.Require().Contains(rendered.String(), testCase.expectedRender)
		})
	}
}

func TestBuilderSuite(t *testing.T) {
	suite.Run(t, new(testSuite))
}
