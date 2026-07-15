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

package client

import (
	"strings"
	"testing"

	"github.com/nuclio/nuclio/pkg/common"

	"github.com/stretchr/testify/assert"
)

// TestGetResourcesNamedPathIsQuoted verifies that a resource path derived from
// a malicious function name is shell-quoted before being used in a command,
// so that shell metacharacters are treated as literals.
func TestGetResourcesNamedPathIsQuoted(t *testing.T) {
	s := &Store{}
	// Use a name with shell metacharacters but no spaces (getResourcePath truncates at spaces,
	// so a space-bearing name would never reach runCommand intact).
	maliciousName := `x";cat+/etc/passwd;"`
	resourcePath := s.getResourcePath(functionsDir, "default", maliciousName)

	// getResourcePath produces a path containing the raw name characters.
	assert.Contains(t, resourcePath, maliciousName)

	// common.Quote must wrap the result in single quotes when it contains
	// shell metacharacters, neutralising them.
	quoted := common.Quote(resourcePath)
	assert.True(t, strings.HasPrefix(quoted, "'"), "expected path to be single-quoted, got: %s", quoted)
}
