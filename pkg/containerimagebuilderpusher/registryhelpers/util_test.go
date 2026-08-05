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

func (suite *UtilTestSuite) TestWriteCredentialFileScriptReferencesEnvVarsOnly() {
	script := writeCredentialFileScript(`az acr login --name "$ACR_REGISTRY_NAME" --expose-token`)

	suite.Equal(`{ echo "$REGISTRY_HOST"; echo "$REGISTRY_USERNAME"; `+
		`az acr login --name "$ACR_REGISTRY_NAME" --expose-token; } > "$REGISTRY_TOKEN_FILE"`, script)
}

func TestUtilTestSuite(t *testing.T) {
	suite.Run(t, new(UtilTestSuite))
}
