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

package functionconfig

import (
	"strings"
	"testing"

	"github.com/ghodss/yaml"
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
      - id: "0"
        checkpoint: "7"
      - id: "1"
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

func (suite *ReaderTestSuite) TestCodeEntryConfigCaseInsensitivity() {
	configData := `
metadata:
  name: code_entry_name
  namespace: code_entry_namespace
spec:
  runtime: python3.6
  handler: code_entry_handler
  targetCpu: 13  # instead of targetCPU to test case insensitivity
`

	config := Config{
		Meta: Meta{
			Name:      "my_name",
			Namespace: "my_namespace",
		},
		Spec: Spec{
			Runtime: "python3.6",
			Handler: "my_handler",
		},
	}
	reader, err := NewReader(suite.logger)
	suite.Require().NoError(err, "Can't create reader")
	err = reader.Read(strings.NewReader(configData), "processor", &config)
	suite.Require().NoError(err, "Can't reader configuration")

	suite.Require().Equal(13, config.Spec.TargetCPU, "Bad target cpu")
}

func (suite *ReaderTestSuite) TestCodeEntryConfigDontOverrideConfigValues() {
	configData := `
metadata:
  name: code_entry_name
  namespace: code_entry_namespace
  labels:
    label_key: label_val
spec:
  runtime: python3.6
  handler: code_entry_handler
  targetCpu: 13
  build:
    commands:
    - pip install code_entry_package
  env:
    - name: env_var
      value: code_entry_env_val
    - name: code_entry_env_var
      value: code_entry_env_val_2
`

	config := Config{
		Meta: Meta{
			Name:      "my_name",
			Namespace: "my_namespace",
			Labels:    map[string]string{}, // empty map
		},
		Spec: Spec{
			Runtime:   "python3.6",
			Handler:   "my_handler",
			Env:       []v1.EnvVar{{Name: "env_var", Value: "my_env_val"}},
			TargetCPU: 51,
		},
	}
	reader, err := NewReader(suite.logger)
	suite.Require().NoError(err, "Can't create reader")
	err = reader.Read(strings.NewReader(configData), "processor", &config)
	suite.Require().NoError(err, "Can't reader configuration")

	suite.Require().Equal("my_name", config.Meta.Name, "Bad name")
	suite.Require().Equal("my_namespace", config.Meta.Namespace, "Bad namespace")

	expectedEnvVariables := []v1.EnvVar{
		{Name: "env_var", Value: "my_env_val"},
		{Name: "code_entry_env_var", Value: "code_entry_env_val_2"},
	}
	suite.Require().Equal(expectedEnvVariables, config.Spec.Env, "Bad env vars")

	suite.Require().Equal("my_handler", config.Spec.Handler, "Bad handler")
	suite.Require().Equal("python3.6", config.Spec.Runtime, "Bad runtime")
	suite.Require().Equal([]string{"pip install code_entry_package"}, config.Spec.Build.Commands, "Bad commands")
	suite.Require().Equal(map[string]string{"label_key": "label_val"}, config.Meta.Labels, "Bad labels")
	suite.Require().Equal(51, config.Spec.TargetCPU, "Bad target cpu")
}

func (suite *ReaderTestSuite) TestCodeEntryConfigTriggerMerge() {
	type TestTrigger struct {
		Name        string
		ServiceType string
	}

	testCases := []struct {
		name                  string
		configTrigger         TestTrigger
		codeEntryTrigger      TestTrigger
		expectedConfigTrigger TestTrigger
		expectValidityError   bool
	}{
		{
			name: "bothNamesDefault",
			configTrigger: TestTrigger{
				Name:        "default-http",
				ServiceType: "ClusterIP",
			},
			codeEntryTrigger: TestTrigger{
				Name:        "default-http",
				ServiceType: "NodePort",
			},
			expectedConfigTrigger: TestTrigger{
				Name:        "default-http",
				ServiceType: "ClusterIP",
			},
			expectValidityError: false,
		},
		{
			name: "codeEntryDefaultNameConfigCustomName",
			configTrigger: TestTrigger{
				Name:        "my-trigger",
				ServiceType: "ClusterIP",
			},
			codeEntryTrigger: TestTrigger{
				Name:        "default-http",
				ServiceType: "NodePort",
			},
			expectedConfigTrigger: TestTrigger{
				Name:        "my-trigger",
				ServiceType: "ClusterIP",
			},
			expectValidityError: false,
		},
		{
			name: "codeEntryCustomNameConfigDefaultName",
			configTrigger: TestTrigger{
				Name:        "default-http",
				ServiceType: "ClusterIP",
			},
			codeEntryTrigger: TestTrigger{
				Name:        "my-trigger",
				ServiceType: "NodePort",
			},
			expectedConfigTrigger: TestTrigger{
				Name:        "my-trigger",
				ServiceType: "NodePort",
			},
			expectValidityError: false,
		},
		{
			name: "bothSameCustomNames",
			configTrigger: TestTrigger{
				Name:        "my-trigger",
				ServiceType: "ClusterIP",
			},
			codeEntryTrigger: TestTrigger{
				Name:        "my-trigger",
				ServiceType: "NodePort",
			},
			expectedConfigTrigger: TestTrigger{
				Name:        "my-trigger",
				ServiceType: "ClusterIP",
			},
			expectValidityError: false,
		},
		{
			name: "differentCustomNames",
			configTrigger: TestTrigger{
				Name:        "my-trigger",
				ServiceType: "ClusterIP",
			},
			codeEntryTrigger: TestTrigger{
				Name:        "not-my-trigger",
				ServiceType: "NodePort",
			},
			expectValidityError: true,
		},
	}

	for _, testCase := range testCases {
		suite.Run(testCase.name, func() {
			config := Config{
				Meta: Meta{
					Name:      "my_name",
					Namespace: "my_namespace",
				},
				Spec: Spec{
					Runtime: "python3.7",
					Handler: "my_handler",
				},
			}
			config.Spec.Triggers = map[string]Trigger{
				testCase.codeEntryTrigger.Name: {
					Name:       testCase.codeEntryTrigger.Name,
					Kind:       "http",
					Attributes: map[string]interface{}{"serviceType": testCase.codeEntryTrigger.ServiceType},
				},
			}
			configData, err := yaml.Marshal(config)
			suite.Require().NoError(err, "Can't marshal config")

			config.Spec.Triggers = map[string]Trigger{
				testCase.configTrigger.Name: {
					Name:       testCase.configTrigger.Name,
					Kind:       "http",
					Attributes: map[string]interface{}{"serviceType": testCase.configTrigger.ServiceType},
				},
			}

			reader, err := NewReader(suite.logger)
			suite.Require().NoError(err)

			err = reader.Read(strings.NewReader(string(configData)), "processor", &config)
			suite.Require().NoError(err)

			err = reader.validateConfigurationFileFunctionConfig(&config)
			if testCase.expectValidityError {
				suite.Require().Error(err)
			} else {
				suite.Require().NoError(err)
				suite.Assert().Equal(1, len(config.Spec.Triggers))
				suite.Assert().Equal(testCase.expectedConfigTrigger.Name,
					config.Spec.Triggers[testCase.expectedConfigTrigger.Name].Name)
				suite.Assert().Equal(testCase.expectedConfigTrigger.ServiceType,
					config.Spec.Triggers[testCase.expectedConfigTrigger.Name].Attributes["serviceType"])
			}
		})
	}
}

func TestRegistryTestSuite(t *testing.T) {
	suite.Run(t, new(ReaderTestSuite))
}
