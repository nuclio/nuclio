//go:build test_unit

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

package registry

import (
	"testing"

	"github.com/stretchr/testify/suite"
)

type RegistryTestSuite struct {
	suite.Suite
}

func (suite *RegistryTestSuite) TestRegistration() {
	r := NewRegistry("myclass")

	kind1Value := 1
	kind2Value := "kind2 value"

	// register two classes
	r.Register("kind1", kind1Value)
	r.Register("kind2", kind2Value)

	// re-registering should panic
	suite.Panics(func() { r.Register("kind1", kind1Value) })

	// get kinds
	kinds := r.GetKinds()
	suite.Require().Len(kinds, 2)
	suite.Require().Contains(kinds, "kind1")
	suite.Require().Contains(kinds, "kind2")

	// get known
	v, err := r.Get("kind1")
	suite.Require().NoError(err)
	suite.Require().Equal(kind1Value, v.(int))

	v, err = r.Get("kind2")
	suite.Require().NoError(err)
	suite.Require().Equal(kind2Value, v.(string))

	// get unknown
	v, err = r.Get("unknown")
	suite.Require().Error(err)
	suite.Require().Nil(v)
}

func TestRegistryTestSuite(t *testing.T) {
	suite.Run(t, new(RegistryTestSuite))
}
