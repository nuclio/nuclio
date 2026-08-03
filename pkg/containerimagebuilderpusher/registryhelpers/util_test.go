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

package registryhelpers

import (
	"testing"

	"github.com/stretchr/testify/suite"
)

type UtilTestSuite struct {
	suite.Suite
}

func (suite *UtilTestSuite) TestWriteCredentialFileScriptEmitsUnquotedSafeTokens() {
	script := writeCredentialFileScript("/tmp/registry-auth-tokens/0.token", "myregistry.azurecr.io",
		"00000000-0000-0000-0000-000000000000", "az acr login --name myregistry --expose-token")

	suite.Equal(`{ echo myregistry.azurecr.io; echo 00000000-0000-0000-0000-000000000000; `+
		`az acr login --name myregistry --expose-token; } > /tmp/registry-auth-tokens/0.token`, script)
}

func (suite *UtilTestSuite) TestWriteCredentialFileScriptQuotesShellMetacharacters() {
	script := writeCredentialFileScript("/tmp/token", "host's; rm -rf /", "user", "cat token")

	suite.Contains(script, `'host'"'"'s; rm -rf /'`)
}

func TestUtilTestSuite(t *testing.T) {
	suite.Run(t, new(UtilTestSuite))
}
