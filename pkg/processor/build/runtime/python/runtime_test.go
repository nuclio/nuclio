//go:build test_unit

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
package python

import (
	"bufio"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/suite"
)

type TestSuite struct {
	suite.Suite
}

func (suite *TestSuite) TestDependenciesAlignment() {
	// mock and pip are excluded from the requirements file as they are not needed to be in post-copy directive
	packages, err := parseRequirementsFile("../../../runtime/python/py/requirements/common.txt",
		[]string{"mock", "pip"})
	suite.Require().NoError(err)

	// Ensure only the expected packages are in the map
	// If there are any additional packages, the test will fail
	expectedPackages := map[string]bool{
		extractPackageName(nuclioSDKRequirement): true,
		extractPackageName(msgPackRequirement):   true,
	}
	suite.Require().Equal(expectedPackages, packages)
}

// parseRequirementsFile reads a requirements file and returns a map of package strings.
// Excluded packages (like mock, pip) will not be included in the map.
func parseRequirementsFile(filePath string, excludeList []string) (map[string]bool, error) {
	// Open the requirements file
	file, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	// Map to store each line
	packages := make(map[string]bool)
	excludeMap := make(map[string]struct{}, len(excludeList))
	for _, ex := range excludeList {
		excludeMap[strings.ToLower(ex)] = struct{}{}
	}

	// Loop over each line in the file
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		// Skip comments and empty lines
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// Extract just the package name (strip version)
		pkgName := extractPackageName(line)
		if _, excluded := excludeMap[strings.ToLower(pkgName)]; excluded {
			continue
		}

		packages[pkgName] = true
	}

	// Check for any scanner errors
	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return packages, nil
}

func extractPackageName(line string) string {
	separators := []string{"==", ">=", "<=", "~=", ">", "<"}
	for _, sep := range separators {
		if parts := strings.Split(line, sep); len(parts) > 1 {
			return strings.ToLower(strings.TrimSpace(parts[0]))
		}
	}
	// If no version constraint, return as-is
	return strings.ToLower(strings.TrimSpace(line))
}

func TestRuntime(t *testing.T) {
	suite.Run(t, new(TestSuite))
}
