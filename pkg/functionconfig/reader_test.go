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

package functionconfig

import (
	"strings"
	"testing"

	"github.com/nuclio/logger"
	"github.com/nuclio/zap"

	"github.com/stretchr/testify/suite"
	"k8s.io/api/core/v1"
)

type ReaderTestSuite struct {
	suite.Suite
	logger logger.Logger
	reader *Reader
}

func (suite *ReaderTestSuite) SetupTest() {
	suite.logger, _ = nucliozap.NewNuclioZapTest("test")
	suite.reader, _ = NewReader(suite.logger)
}

func (suite *ReaderTestSuite) TestPartitions() {
	configData := `
metadata:
  name: python handler
spec:
  runtime: python
  handler: reverser:handler
  triggers:
    http:
      maxWorkers: 4
      kind: http
    franz:
      kind: "kafka"
      url: "127.0.0.1:9092"
      total_tasks: 2
      max_task_allocation: 3
      partitions:
      - id: 0
        checkpoint: 7
      - id: 1
      attributes:
        topic: trial
`

	config := Config{}
	reader, err := NewReader(suite.logger)
	suite.Require().NoError(err, "Can't create reader")
	err = reader.Read(strings.NewReader(configData), "processor", &config)
	suite.Require().NoError(err, "Can't reader configuration")

	trigger := config.Spec.Triggers["franz"]
	suite.Require().Equal(2, trigger.TotalTasks, "Bad total_tasks")
	suite.Require().Equal(3, trigger.MaxTaskAllocation, "Bad max_task_allocations")

	suite.Require().Equal(2, len(trigger.Partitions), "Wrong number of partitions")
	for _, partition := range trigger.Partitions {
		switch partition.ID {
		case "0":
			suite.Require().Equal("7", *partition.Checkpoint, "Bad checkpoint")
		case "1":
			suite.Require().Nil(partition.Checkpoint)
		default:
			suite.Require().Failf("Unknown partition ID - %s", partition.ID)
		}
	}
}

func (suite *ReaderTestSuite) TestReceivedConfigDontOverrideBaseConfig() {
	configData := `
metadata:
  name: new_name
  namespace: new_namespace
  labels:
    label_key: label_val
spec:
  runtime: python3.6
  handler: new_handler
  build:
    commands:
    - pip install new
  env:
    - name: env_var
      value: new_env_val
    - name: new_env_var
      value: new_env_val_2
`

	baseConfig := Config{
		Meta: Meta{
			Name: "base_name",
			Namespace: "base_namespace",
			Labels: map[string]string{}, // empty map
		},
		Spec: Spec{
			Runtime: "python2.7",
			Handler: "base_handler",
			Env: []v1.EnvVar{{Name: "env_var", Value: "base_env_val"}},
		},
	}
	reader, err := NewReader(suite.logger)
	suite.Require().NoError(err, "Can't create reader")
	err = reader.Read(strings.NewReader(configData), "processor", &baseConfig)
	suite.Require().NoError(err, "Can't reader configuration")

	suite.Require().Equal("base_name", baseConfig.Meta.Name, "Bad name")
	suite.Require().Equal("base_namespace", baseConfig.Meta.Namespace, "Bad namespace")

	expectedEnvVariables := []v1.EnvVar{
		{Name: "env_var", Value: "base_env_val"},
		{Name: "new_env_var", Value: "new_env_val_2"},
	}
	suite.Require().Equal(expectedEnvVariables, baseConfig.Spec.Env, "Bad env vars")

	suite.Require().Equal("base_handler", baseConfig.Spec.Handler, "Bad handler")
	suite.Require().Equal("python2.7", baseConfig.Spec.Runtime, "Bad runtime")
	suite.Require().Equal([]string{"pip install new"}, baseConfig.Spec.Build.Commands, "Bad commands")
	suite.Require().Equal(map[string]string{"label_key": "label_val"}, baseConfig.Meta.Labels, "Bad labels")
}

func (suite *ReaderTestSuite) TestToDeployOptions() {
	suite.T().Skip("TODO")
	//	flatConfigurationContents := `
	//
	//name: function-name
	//namespace: function-namespace
	//runtime: golang:1.10
	//handler: some.module:handler
	//triggers:
	//
	//  http:
	//    maxWorkers: 4
	//    kind: http
	//
	//  rmq:
	//    kind: rabbit-mq
	//    url: amqp://guest:guest@34.224.60.166:5672
	//    attributes:
	//      exchangeName: functions
	//      queueName: functions
	//
	//dataBindings:
	//  db0:
	//    class: v3io
	//    secret: something
	//    url: http://192.168.51.240:8081/1024
	//
	//build:
	//  commands:
	//  - command1
	//  - command2
	//  - command3
	//  baseImage: someBaseImage
	//`

	//createFunctionOptions := platform.NewDeployOptions(nil)
	//
	//err := suite.reader.Read(bytes.NewBufferString(flatConfigurationContents), "yaml")
	//suite.Require().NoError(err)
	//
	//err = suite.reader.ToDeployOptions(createFunctionOptions)
	//suite.Require().NoError(err)
	//

	// compare.CompareNoOrder(&createFunctionOptions, &createFunctionOptions)
	// TODO
}

func TestRegistryTestSuite(t *testing.T) {
	suite.Run(t, new(ReaderTestSuite))
}
