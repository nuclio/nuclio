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

package common

import (
	"testing"

	"github.com/stretchr/testify/suite"
)

type MapTestSuite struct {
	suite.Suite
}

func (ts *MapTestSuite) TestMapStringInterfaceGetOrDefault() {
	source := map[string]interface{}{
		"k_str1":  "str1",
		"k_str2":  "str2",
		"k_int1":  1,
		"k_bool1": true,
	}

	vs := MapStringInterfaceGetOrDefault(source, "k_str1", "default").(string)
	ts.Require().Equal("str1", vs)

	vs = MapStringInterfaceGetOrDefault(source, "k_str2", "default").(string)
	ts.Require().Equal("str2", vs)

	vs = MapStringInterfaceGetOrDefault(source, "k_str1__", "default").(string)
	ts.Require().Equal("default", vs)

	vi := MapStringInterfaceGetOrDefault(source, "k_int1", 100).(int)
	ts.Require().Equal(1, vi)

	vi = MapStringInterfaceGetOrDefault(source, "k_int1__", 100).(int)
	ts.Require().Equal(100, vi)

	vi = MapStringInterfaceGetOrDefault(source, "k_str2", 1).(int)
	ts.Require().Equal(1, vi)

	vb := MapStringInterfaceGetOrDefault(source, "k_bool1", false).(bool)
	ts.Require().Equal(true, vb)
}

func TestMapTestSuite(t *testing.T) {
	suite.Run(t, new(MapTestSuite))
}
