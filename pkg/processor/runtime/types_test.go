/*
Copyright 2017 The Nuclio Authors.

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

package runtime

import (
	"testing"

	"fmt"
	"github.com/nuclio/nuclio/pkg/functioncr"
	"github.com/stretchr/testify/suite"
)

type TypesTestSuite struct {
	suite.Suite
}

func (suite *TypesTestSuite) TestGetDataBindingsFromEnv() {
	c := Configuration{}

	dataBindings := map[string]*functioncr.DataBinding{}

	env := []string{
		"IGNORE=ME",
		"IGNOREME=TOO",
		"NUCLIO_DATA_BINDING_some_binding_CLASS=some_binding_class",
		"NUCLIO_DATA_BINDING_some_binding_URL=some_binding_url",
		"NUCLIO_DATA_BINDING_another_CLASS=another_class",
		"NUCLIO_DATA_BINDING_another_URL=another_url",
	}

	c.getDatabindingsFromEnv(env, dataBindings)

	// TODO: verify
}

func TestTypesTestSuite(t *testing.T) {
	suite.Run(t, new(TypesTestSuite))
}
