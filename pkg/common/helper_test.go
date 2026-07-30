//go:build test_unit

/*
Copyright 2023 The Nuclio Authors.

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

package common

import (
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"
)

type IsFileTestSuite struct {
	suite.Suite
	tempDir string
}

func (suite *IsFileTestSuite) SetupSuite() {
	var err error

	// Create temp dir for IsFileTestSuite
	suite.tempDir, err = os.MkdirTemp("", "isfile-test")
	suite.Require().NoError(err)
}

func (suite *IsDirTestSuite) TearDownSuite() {
	defer os.RemoveAll(suite.tempDir)
}

func (suite *IsFileTestSuite) TestPositive() {

	// Create temp file
	tempFile, err := os.CreateTemp(suite.tempDir, "temp_file")
	suite.Require().NoError(err)
	defer os.Remove(tempFile.Name())

	// Verify that function isFile() returns true when file is created
	suite.Require().True(IsFile(tempFile.Name()))

}

func (suite *IsFileTestSuite) TestFileIsNotExist() {

	// Set path to unexisted file
	tempFile := filepath.Join(suite.tempDir, "somePath.txt")

	// Verify that function isFile() returns false when file doesn't exist in the system
	suite.Require().False(IsFile(tempFile))
}

func (suite *IsFileTestSuite) TestFileIsADirectory() {
	suite.Require().False(IsFile(suite.tempDir))
}

type IsDirTestSuite struct {
	suite.Suite
	tempDir string
}

func (suite *IsDirTestSuite) SetupSuite() {
	var err error

	// Create temp dir for IsDirTestSuite
	suite.tempDir, err = os.MkdirTemp("", "isdir-test")
	suite.Require().NoError(err)
}

func (suite *IsFileTestSuite) TearDownSuite() {
	defer os.RemoveAll(suite.tempDir)
}

func (suite *IsDirTestSuite) TestPositive() {

	// Verify that function IsDir() returns true when directory exists in the system
	suite.Require().True(IsDir(suite.tempDir))
}

func (suite *IsDirTestSuite) TestNegative() {

	// Create temp file
	tempFile, err := os.CreateTemp(suite.tempDir, "temp_file")
	suite.Require().NoError(err)
	defer os.Remove(tempFile.Name())

	// Verify that function IsDir( returns false when file instead of directory is function argument
	suite.Require().False(IsDir(tempFile.Name()))
}

type FileExistTestSuite struct {
	suite.Suite
	tempDir string
}

func (suite *FileExistTestSuite) SetupSuite() {
	var err error

	// Create temp dir for FileExistTestSuite
	suite.tempDir, err = os.MkdirTemp("", "file_exists-test")
	suite.Require().NoError(err)
}

func (suite *FileExistTestSuite) TearDownSuite() {
	defer os.RemoveAll(suite.tempDir)
}

func (suite *FileExistTestSuite) TestPositive() {

	// Create temp file
	tempFile, err := os.CreateTemp(suite.tempDir, "temp_file")
	suite.Require().NoError(err)
	defer os.Remove(tempFile.Name())

	// Verify that function FileExists() returns true when file is exist
	suite.Require().True(FileExists(tempFile.Name()))
}

func (suite *FileExistTestSuite) TestFileNotExist() {

	// Set path to unexisted file
	tempFile := filepath.Join(suite.tempDir, "somePath.txt")

	// Verify that function FileExists() returns false when file doesn't exist
	suite.Require().False(FileExists(tempFile))
}

func (suite *FileExistTestSuite) TestFileIsNotAFile() {

	// Verify that function returns true when folder is exist in the system
	suite.Require().True(FileExists(suite.tempDir))
}

type StringSliceToIntSliceTestSuite struct {
	suite.Suite
}

func (suite *StringSliceToIntSliceTestSuite) TestPositive() {

	// Prepare slice for StringSliceToIntSlice() function
	stringSlice := []string{"1", "2", "5", "6", "23"}
	expectedSlice := []int{1, 2, 5, 6, 23}
	actualSlice, err := StringSliceToIntSlice(stringSlice)

	// Check that slice successfully casted into []int slice
	suite.Require().NoError(err)
	suite.Require().Equal(expectedSlice, actualSlice)
}

func (suite *StringSliceToIntSliceTestSuite) TestNegativeData() {

	// Prepare incorrect (for casting) slice for StringSliceToIntSlice() function
	stringSlice := []string{"1", "2", "5", "6", "23", "someBadData"}
	_, err := StringSliceToIntSlice(stringSlice)

	// Verify that error is throws by StringSliceToIntSlice() function
	suite.Require().Error(err)
}

type RetryUntilSuccessfulOnErrorPatternsTestSuite struct {
	suite.Suite
}

func (suite *RetryUntilSuccessfulOnErrorPatternsTestSuite) TestSucceedIfErrorMessageIsEmpty() {
	var calls int
	for _, testCase := range []struct {
		description    string
		expectedCalls  int
		callbackErrors []string
		errorPatterns  []string
		shouldFail     bool

		// on timeout error we dont assert call count since we cannot anticipate its counter
		shouldTimeout bool
	}{
		{
			description:   "Succeeded after 2 retries",
			expectedCalls: 3,
			callbackErrors: []string{
				"First",
				"Second failure",
				"",
			},
			errorPatterns: []string{
				"^First$",
				"Second",
			},
			shouldFail: false,
		},
		{
			description:   "Succeeded after 1 call when callback error is empty",
			expectedCalls: 1,
			callbackErrors: []string{
				"",
			},
			errorPatterns: []string{
				"dont-care",
			},
			shouldFail: false,
		},
		{
			description:   "Succeeded after 1 call when callback error is empty",
			expectedCalls: 1,
			callbackErrors: []string{
				"",
			},
			errorPatterns: []string{
				"dont-care",
			},
			shouldFail: false,
		},
		{
			description:   "Failed after 1 call due to unmatched error",
			expectedCalls: 1,
			callbackErrors: []string{
				"A",
				"B",
				"C",
			},
			errorPatterns: []string{
				"^That$",
			},
			shouldFail: true,
		},
		{
			description: "Failed due to timeout",
			callbackErrors: []string{
				"A",
			},
			errorPatterns: []string{
				"^A",
			},
			shouldFail:    true,
			shouldTimeout: true,
		},
	} {
		calls = 0
		err := RetryUntilSuccessfulOnErrorPatterns(
			50*time.Millisecond,
			10*time.Millisecond,
			testCase.errorPatterns,
			func(int) (string, error) {
				errorMessage := testCase.callbackErrors[calls]
				if !testCase.shouldTimeout {
					calls++
				}
				return errorMessage, nil
			})
		if testCase.shouldFail {
			suite.Error(err)
		} else {
			suite.NoError(err)
		}

		if !testCase.shouldTimeout {
			suite.Equal(testCase.expectedCalls, calls)
		}
	}

}

type RetryUntilSuccessfulTestSuite struct {
	suite.Suite
}

func (suite *RetryUntilSuccessfulTestSuite) TestPositive() {
	err := RetryUntilSuccessful(50*time.Millisecond, 10*time.Millisecond, func() bool {
		return true
	})

	suite.Require().NoError(err)
}

func (suite *RetryUntilSuccessfulTestSuite) TestNegative() {
	err := RetryUntilSuccessful(50*time.Millisecond, 10*time.Millisecond, func() bool {
		return false
	})

	suite.Require().Error(err)

}

func (suite *RetryUntilSuccessfulTestSuite) TestNumberOfCalls() {

	// Create actual and expected number of calls
	actualNumberOfCalls := 0
	expectedNumberOfCalls := 10

	_ = RetryUntilSuccessful(1000*time.Millisecond, 100*time.Millisecond, func() bool {
		_, _, _, ok := runtime.Caller(1)
		if ok {
			actualNumberOfCalls++
		}
		return false
	})

	suite.Require().Equal(expectedNumberOfCalls, actualNumberOfCalls)
}

func (suite *RetryUntilSuccessfulTestSuite) TestTimeBetweenIntervals() {

	// Starting time from currentTime - 100ms cause function calls right now
	startingIntervalTime := getCurrentTimeInMilliseconds() - 100
	_ = RetryUntilSuccessful(1000*time.Millisecond, 100*time.Millisecond, func() bool {
		_, _, _, ok := runtime.Caller(1)
		if ok {

			// If call was successful create finishIntervalTime variable and set currentTime
			finishIntervalTime := getCurrentTimeInMilliseconds()

			// Verify that difference between previous interval and current interval is from 60 to 120ms
			suite.Require().True((finishIntervalTime-startingIntervalTime > 60) && (finishIntervalTime-startingIntervalTime < 120))

			// Set currentInterval time value into previous interval variable
			startingIntervalTime = finishIntervalTime
		}
		return false
	})
}

func (suite *RetryUntilSuccessfulTestSuite) TestDurationTime() {

	// Initialize startTime as currentTime
	startTime := getCurrentTimeInMilliseconds()
	_ = RetryUntilSuccessful(1000*time.Millisecond, 100*time.Millisecond, func() bool {
		return false
	})

	// Initialize finishTime as currentTime
	finishTime := getCurrentTimeInMilliseconds()

	// Verify that function duration is as expected
	suite.Require().True((finishTime-startTime > 960) && (finishTime-startTime < 1060))
}

func getCurrentTimeInMilliseconds() int64 {
	return time.Now().UnixNano() / int64(time.Millisecond)
}

type StripPrefixesTestSuite struct {
	suite.Suite
}

func (suite *StripPrefixesTestSuite) TestPositive() {
	stripped := StripPrefixes("prefix_something_1", []string{"prefix_"})
	suite.Require().Equal("something_1", stripped)

	stripped = StripPrefixes("prefix_something_1", []string{"not_prefix", "prefix_"})
	suite.Require().Equal("something_1", stripped)

	stripped = StripPrefixes("prefix_something_1", []string{"prefix_", "not_prefix", "not_prefix_2"})
	suite.Require().Equal("something_1", stripped)

	stripped = StripPrefixes("prefix_something_1", []string{"not_prefix", "not_prefix_2"})
	suite.Require().Equal("prefix_something_1", stripped)
}

type LabelsMapMatcherTestSuite struct {
	suite.Suite
}

func (suite *LabelsMapMatcherTestSuite) Test() {
	for _, testCase := range []struct {
		name                 string
		labelsMap            map[string]string
		encodedLabelSelector string
		matching             bool
		expectedError        bool
	}{
		{
			name: "Sanity",
			labelsMap: map[string]string{
				"c": "d",
			},
			encodedLabelSelector: "c=d",
			matching:             true,
		},
		{
			name: "EmptyLabelSelectorsMatchAll",
			labelsMap: map[string]string{
				"a": "b",
			},
			encodedLabelSelector: "",
			matching:             true,
		},
		{
			name:                 "NillableLabelMaps",
			labelsMap:            nil,
			encodedLabelSelector: "",
			matching:             true,
		},

		// miss match
		{
			name: "EncodedLabelSelectorsNotInLabels",
			labelsMap: map[string]string{
				"a": "b",
				"c": "d",
			},
			encodedLabelSelector: "z=w",
			matching:             false,
		},
		{
			name:      "EncodedLabelSelectorsNilLabels",
			labelsMap: nil,

			encodedLabelSelector: "a=b",
			matching:             false,
		},

		// explode
		{
			name:                 "InvalidEncodedLabelSelector",
			labelsMap:            nil,
			encodedLabelSelector: "!@#$",
			expectedError:        true,
		},
	} {
		suite.Run(testCase.name, func() {
			matching, err := LabelsMapMatchByLabelSelector(testCase.encodedLabelSelector, testCase.labelsMap)
			if testCase.expectedError {
				suite.Require().Error(err)
				return
			}
			suite.Require().NoError(err)
			suite.Require().Equal(testCase.matching, matching)
		})

	}
}

type MiscTestSuite struct {
	suite.Suite
}

func (suite *MiscTestSuite) TestPopulateFieldsFromValues() {
	type testObject struct {
		stringField1 string
		stringField2 string
		intField1    int
		intField2    int
		boolField1   bool
		boolField2   bool
	}
	object := testObject{}

	for _, testCase := range []struct {
		name                 string
		kind                 string
		stringFieldsToValues map[*string]string
		intFieldsToValues    map[*int]int
		boolFieldsToValues   map[*bool]bool
		initialObject        testObject
		expectedObject       testObject
	}{
		{
			name: "StringEmptyFields",
			kind: "string",
			stringFieldsToValues: map[*string]string{
				&object.stringField1: "stringField1",
				&object.stringField2: "stringField2",
			},
			initialObject: testObject{},
			expectedObject: testObject{
				stringField1: "stringField1",
				stringField2: "stringField2",
			},
		},
		{
			name: "StringNonEmptyFields",
			kind: "string",
			stringFieldsToValues: map[*string]string{
				&object.stringField1: "stringField1",
				&object.stringField2: "stringField2",
			},
			initialObject: testObject{
				stringField1: "nonEmptyStringField1",
			},
			expectedObject: testObject{
				stringField1: "nonEmptyStringField1",
				stringField2: "stringField2",
			},
		},
		{
			name: "IntEmptyFields",
			kind: "int",
			intFieldsToValues: map[*int]int{
				&object.intField1: 1,
				&object.intField2: 5,
			},
			initialObject: testObject{},
			expectedObject: testObject{
				intField1: 1,
				intField2: 5,
			},
		},
		{
			name: "IntNonEmptyFields",
			kind: "int",
			intFieldsToValues: map[*int]int{
				&object.intField1: 1,
				&object.intField2: 5,
			},
			initialObject: testObject{
				intField1: 2,
			},
			expectedObject: testObject{
				intField1: 2,
				intField2: 5,
			},
		},
		{
			name: "BoolEmptyFields",
			kind: "bool",
			boolFieldsToValues: map[*bool]bool{
				&object.boolField1: false,
				&object.boolField2: true,
			},
			initialObject: testObject{},
			expectedObject: testObject{
				boolField1: false,
				boolField2: true,
			},
		},
		{
			name: "BoolNonEmptyFields",
			kind: "bool",
			boolFieldsToValues: map[*bool]bool{
				&object.boolField1: false,
				&object.boolField2: true,
			},
			initialObject: testObject{
				boolField1: true,
			},
			expectedObject: testObject{
				boolField1: true,
				boolField2: true,
			},
		},
	} {
		suite.Run(testCase.name, func() {
			object = testCase.initialObject

			switch testCase.kind {
			case "string":
				PopulateFieldsFromValues(testCase.stringFieldsToValues)
			case "int":
				PopulateFieldsFromValues(testCase.intFieldsToValues)
			case "bool":
				PopulateFieldsFromValues(testCase.boolFieldsToValues)
			}
			suite.Require().Equal(testCase.expectedObject, object)

			// cleanup object
			object = testObject{}
		})
	}
}

func (suite *MiscTestSuite) TestSanitizeResponseData() {
	for _, testCase := range []struct {
		name           string
		data           string
		header         http.Header
		expectedResult string
	}{
		{
			name:           "EmptyString",
			data:           "",
			header:         http.Header{},
			expectedResult: "",
		},
		{
			name: "ValidString",
			data: "some data",
			header: http.Header{
				"Content-Type": []string{"text/plain"},
			},
			expectedResult: "some data",
		},
		{
			name: "Integers",
			data: "123",
			header: http.Header{
				"Content-Type": []string{"text/plain"},
			},
			expectedResult: "123",
		},
		{
			name: "json",
			data: `{"key": "value"}`,
			header: http.Header{
				"Content-Type": []string{"application/json"},
			},
			expectedResult: `{"key": "value"}`,
		},
		{
			name: "JavaScript",
			data: "<script>alert('XSS')</script>",
			header: http.Header{
				"Content-Type": []string{"text/javascript"},
			},
			expectedResult: "",
		},
		{
			name: "HTMLWithEvilElements",
			data: "<a href='javascript:alert(1)'>Click me</a>",
			header: http.Header{
				"Content-Type": []string{"text/html"},
			},
			expectedResult: "Click me",
		},
		{
			name: "HTMLWithRegularElements1",
			data: "<p>Hello, <b>world</b>!</p>",
			header: http.Header{
				"Content-Type": []string{"text/html"},
			},
			expectedResult: "<p>Hello, <b>world</b>!</p>",
		},
		{
			name: "HTMLWithRegularElements2",
			data: "Hello, <b>world</b>!",
			header: http.Header{
				"Content-Type": []string{"text/plain"},
			},
			expectedResult: "Hello, <b>world</b>!",
		},
	} {
		suite.Run(testCase.name, func() {
			sanitizedData := SanitizeResponseData([]byte(testCase.data), testCase.header)
			suite.Require().Equal(testCase.expectedResult, string(sanitizedData))
		})
	}
}

func (suite *MiscTestSuite) TestImageHasRegistry() {
	for _, testCase := range []struct {
		name     string
		image    string
		expected bool
	}{
		{
			name:     "emptyString",
			image:    "",
			expected: false,
		},
		{
			name:     "onlyImageName",
			image:    "nginx",
			expected: false,
		},
		{
			name:     "imageWithTag",
			image:    "nginx:latest",
			expected: false,
		},
		{
			name:     "imageWithRegistry",
			image:    "docker.io/nginx:latest",
			expected: true,
		},
		{
			name:     "imageWithRegistryAndPort",
			image:    "registry.example.com:5000/repo/nginx:latest",
			expected: true,
		},
		{
			name:     "imageWithLeadingSlash",
			image:    "/nginx:latest",
			expected: false,
		},
		{
			name:     "imageWithOrgAndName",
			image:    "library/nginx",
			expected: false,
		},
		{
			name:     "imageWithNestedPath",
			image:    "ghcr.io/org/project/nginx:1.0",
			expected: true,
		},
	} {
		suite.Run(testCase.name, func() {
			suite.Require().Equal(testCase.expected, ImageHasRegistry(testCase.image))
		})
	}
}

type IsPathWithinDirTestSuite struct {
	suite.Suite
}

func (suite *IsPathWithinDirTestSuite) TestIsPathWithinDir() {
	const dir = "/tmp/nuclio-build-123/source"
	for _, testCase := range []struct {
		name       string
		targetPath string
		expected   bool
	}{
		{
			name:       "childWithinDir",
			targetPath: "/tmp/nuclio-build-123/source/main.go",
			expected:   true,
		},
		{
			name:       "dirItself",
			targetPath: "/tmp/nuclio-build-123/source",
			expected:   false,
		},
		{
			name:       "exactlyParentDir",
			targetPath: "/tmp/nuclio-build-123",
			expected:   false,
		},
		{
			// the advisory reproducer: "../" sequences escaping the build dir
			name:       "parentEscape",
			targetPath: "/tmp/nuclio-build-123/source/../../../../tmp/evil.txt",
			expected:   false,
		},
		{
			// guards against a naive HasPrefix(path, dir) check, which would wrongly
			// accept a sibling dir sharing the prefix ("source-evil" vs "source")
			name:       "siblingWithSharedPrefix",
			targetPath: "/tmp/nuclio-build-123/source-evil/x.txt",
			expected:   false,
		},
	} {
		suite.Run(testCase.name, func() {
			within, err := IsPathWithinDir(testCase.targetPath, dir)
			suite.Require().NoError(err)
			suite.Require().Equal(testCase.expected, within)
		})
	}
}

type ContainsPathTraversalTestSuite struct {
	suite.Suite
}

func (suite *ContainsPathTraversalTestSuite) TestContainsPathTraversal() {
	for _, testCase := range []struct {
		name     string
		path     string
		expected bool
	}{
		{
			name:     "cleanAbsolutePath",
			path:     "/tmp/nuclio-build-123/source",
			expected: false,
		},
		{
			name:     "cleanRelativePath",
			path:     "handler/main.go",
			expected: false,
		},
		{
			name:     "emptyPath",
			path:     "",
			expected: false,
		},
		{
			name:     "leadingTraversal",
			path:     "../../etc/passwd",
			expected: true,
		},
		{
			name:     "embeddedTraversal",
			path:     "/tmp/nuclio-build-123/source/../../../../etc/passwd",
			expected: true,
		},
		{
			// removing "../" alone would leave "../", so the whole input must be rejected
			name:     "obfuscatedTraversal",
			path:     "....//etc/passwd",
			expected: true,
		},
	} {
		suite.Run(testCase.name, func() {
			suite.Require().Equal(testCase.expected, ContainsPathTraversal(testCase.path))
		})
	}
}

type EnvWithLegacyKeyTestSuite struct {
	suite.Suite
}

func (suite *EnvWithLegacyKeyTestSuite) TestGetEnvOrDefaultStringWithLegacyKey() {
	for _, testCase := range []struct {
		name        string
		value       string
		legacyValue string
		setValue    bool
		setLegacy   bool
		expected    string
	}{
		{name: "NeitherSet", expected: "default"},
		{name: "OnlyLegacySet", setLegacy: true, legacyValue: "legacy", expected: "legacy"},
		{name: "OnlyNewSet", setValue: true, value: "new", expected: "new"},
		{name: "BothSetNewWins", setValue: true, value: "new", setLegacy: true, legacyValue: "legacy", expected: "new"},
	} {
		suite.Run(testCase.name, func() {
			if testCase.setValue {
				suite.T().Setenv("TEST_NEW_KEY", testCase.value)
			}
			if testCase.setLegacy {
				suite.T().Setenv("TEST_LEGACY_KEY", testCase.legacyValue)
			}

			suite.Equal(testCase.expected,
				GetEnvOrDefaultStringWithLegacyKey("TEST_NEW_KEY", "TEST_LEGACY_KEY", "default"))
		})
	}
}

func (suite *EnvWithLegacyKeyTestSuite) TestGetEnvOrDefaultBoolWithLegacyKey() {
	for _, testCase := range []struct {
		name        string
		value       string
		legacyValue string
		setValue    bool
		setLegacy   bool
		expected    bool
	}{
		{name: "NeitherSet", expected: false},
		{name: "OnlyLegacySet", setLegacy: true, legacyValue: "true", expected: true},
		{name: "OnlyNewSet", setValue: true, value: "true", expected: true},
		{name: "BothSetNewWins", setValue: true, value: "false", setLegacy: true, legacyValue: "true", expected: false},
	} {
		suite.Run(testCase.name, func() {
			if testCase.setValue {
				suite.T().Setenv("TEST_NEW_BOOL_KEY", testCase.value)
			}
			if testCase.setLegacy {
				suite.T().Setenv("TEST_LEGACY_BOOL_KEY", testCase.legacyValue)
			}

			suite.Equal(testCase.expected,
				GetEnvOrDefaultBoolWithLegacyKey("TEST_NEW_BOOL_KEY", "TEST_LEGACY_BOOL_KEY", false))
		})
	}
}

type StripImageTagTestSuite struct {
	suite.Suite
}

func (suite *StripImageTagTestSuite) TestStripImageTag() {
	for _, testCase := range []struct {
		name     string
		image    string
		expected string
	}{
		{
			name:     "tagAfterSlash",
			image:    "registry.example.com/my-func:latest",
			expected: "registry.example.com/my-func",
		},
		{
			name:     "tagWithoutRegistry",
			image:    "my-func:some-tag",
			expected: "my-func",
		},
		{
			name:     "noTag",
			image:    "registry.example.com/my-func",
			expected: "registry.example.com/my-func",
		},
		{
			name:     "portInHostNoTag",
			image:    "registry.example.com:5000/my-func",
			expected: "registry.example.com:5000/my-func",
		},
	} {
		suite.Run(testCase.name, func() {
			suite.Require().Equal(testCase.expected, StripImageTag(testCase.image))
		})
	}
}

type SanitizeKubernetesNameTestSuite struct {
	suite.Suite
}

func (suite *SanitizeKubernetesNameTestSuite) TestSanitizeKubernetesName() {
	for _, testCase := range []struct {
		name            string
		prefix          string
		value           string
		forGenerateName bool
		expected        string
	}{
		{
			name:     "LowercasesAndReplacesInvalidChars",
			value:    "Registry.Example.Com/My_Func:v1.2",
			expected: "registry-example-com-my-func-v1-2",
		},
		{
			name:     "RunOfInvalidCharsBecomesOneDash",
			value:    "a...b",
			expected: "a-b",
		},
		{
			name:     "LeadingAndTrailingDashesTrimmed",
			value:    "--a-b--",
			expected: "a-b",
		},
		{
			name:     "PrefixUsedVerbatim",
			prefix:   "registry-login-azure-",
			value:    "myregistry.azurecr.io",
			expected: "registry-login-azure-myregistry-azurecr-io",
		},
		{
			name:     "EmptyValueLeavesNoDanglingDash",
			prefix:   "registry-login-aws-",
			value:    "///:::",
			expected: "registry-login-aws",
		},
		{
			name:     "TruncatedToLabelLimit",
			prefix:   "registry-login-aws-",
			value:    strings.Repeat("a", 80),
			expected: "registry-login-aws-" + strings.Repeat("a", KubernetesDomainLevelMaxLength-len("registry-login-aws-")),
		},
		{
			name:            "GenerateNameAppendsDashAndReservesSuffix",
			prefix:          "nuclio-buildjob-",
			value:           "my-func:latest",
			forGenerateName: true,
			expected:        "nuclio-buildjob-my-func-latest-",
		},
		{
			name:            "GenerateNameTruncatesLeavingRoomForSuffix",
			prefix:          "nuclio-buildjob-",
			value:           strings.Repeat("x", 80),
			forGenerateName: true,
			expected:        "nuclio-buildjob-" + strings.Repeat("x", 41) + "-",
		},
		{
			name:            "GenerateNameTruncationLandingOnDashIsTrimmed",
			prefix:          "nuclio-buildjob-",
			value:           strings.Repeat("a", 41) + "." + strings.Repeat("b", 40),
			forGenerateName: true,
			expected:        "nuclio-buildjob-" + strings.Repeat("a", 41) + "-",
		},
	} {
		suite.Run(testCase.name, func() {
			result, err := SanitizeKubernetesName(testCase.prefix, testCase.value, testCase.forGenerateName)

			suite.Require().NoError(err)
			suite.Equal(testCase.expected, result)

			// the result must always fit a Kubernetes name, including the generated random suffix
			totalLength := len(result)
			if testCase.forGenerateName {
				totalLength += generateNameRandomSuffixLength
			}
			suite.LessOrEqual(totalLength, KubernetesDomainLevelMaxLength)
		})
	}
}

func (suite *SanitizeKubernetesNameTestSuite) TestSanitizeKubernetesNameErrorsOnInvalidPrefix() {
	for _, prefix := range []string{"Nuclio-", "nuclio_", "nuclio/", "nuclio ", "has.dot-"} {
		_, err := SanitizeKubernetesName(prefix, "value", false)
		suite.Require().Error(err, "prefix %q should be rejected", prefix)
	}
}

func (suite *SanitizeKubernetesNameTestSuite) TestSanitizeKubernetesNameAcceptsValidPrefix() {
	for _, prefix := range []string{"", "nuclio-", "a-b-c-", "abc123"} {
		_, err := SanitizeKubernetesName(prefix, "value", false)
		suite.Require().NoError(err, "prefix %q should be accepted", prefix)
	}
}

func TestHelperTestSuite(t *testing.T) {
	suite.Run(t, new(RetryUntilSuccessfulTestSuite))
	suite.Run(t, new(RetryUntilSuccessfulOnErrorPatternsTestSuite))
	suite.Run(t, new(StringSliceToIntSliceTestSuite))
	suite.Run(t, new(FileExistTestSuite))
	suite.Run(t, new(IsDirTestSuite))
	suite.Run(t, new(IsFileTestSuite))
	suite.Run(t, new(StripPrefixesTestSuite))
	suite.Run(t, new(LabelsMapMatcherTestSuite))
	suite.Run(t, new(MiscTestSuite))
	suite.Run(t, new(IsPathWithinDirTestSuite))
	suite.Run(t, new(ContainsPathTraversalTestSuite))
	suite.Run(t, new(EnvWithLegacyKeyTestSuite))
	suite.Run(t, new(StripImageTagTestSuite))
	suite.Run(t, new(SanitizeKubernetesNameTestSuite))
}
