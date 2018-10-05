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

package mqtt

import (
	"testing"

	"github.com/nuclio/nuclio/pkg/processor/test/suite"

	"github.com/stretchr/testify/suite"
)

type TestSuite struct {
	processorsuite.TestSuite
	trigger mqtt
}

func (suite *TestSuite) SetupSuite() {
	suite.TestSuite.SetupSuite()
}

func (suite *TestSuite) TearDownSuite() {
	suite.TestSuite.TearDownTest()
}

func (suite *TestSuite) SetupTest() {
	suite.trigger = mqtt{}
	suite.trigger.Logger = suite.Logger.GetChild("mqtt")

	suite.trigger.configuration = &Configuration{
	}
}

func (suite *TestSuite) TestSetEmptyParametersNoChange() {
}

func TestRabbitMQSuite(t *testing.T) {
	suite.Run(t, new(TestSuite))
}
