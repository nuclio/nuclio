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

package platform

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/suite"
)

type TypesTestSuite struct {
	suite.Suite
}

func (suite *TypesTestSuite) TestFlatConfiguration() {
	flatConfigurationContents := `
name: function-name
namespace: function-namespace
runtime: golang:1.9
handler: some.module:handler
triggers:

  http:
    maxWorkers: 4
    kind: http

  rmq:
    kind: rabbit-mq
    url: amqp://guest:guest@34.224.60.166:5672
    attributes:
      exchangeName: functions
      queueName: functions

dataBindings:
  db0:
    class: v3io
    secret: something
    url: http://192.168.51.240:8081/1024

build:
  commands:
  - command1
  - command2
  - command3
  baseImageName: someBaseImage
`

	deployOptions := DeployOptions{}

	err := deployOptions.ReadFunctionConfig(bytes.NewBufferString(flatConfigurationContents))
	suite.Require().NoError(err)

	// compare.CompareNoOrder(&deployOptions, &deployOptions)
	// TODO
}

func TestRegistryTestSuite(t *testing.T) {
	suite.Run(t, new(TypesTestSuite))
}
