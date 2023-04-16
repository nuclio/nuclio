//go:build test_unit

/*
Copyright 2018 The Nuclio Authors.

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

package kafka

import (
	"os"
	"path"
	"testing"

	"github.com/nuclio/nuclio/pkg/processor/trigger"

	"github.com/nuclio/logger"
	nucliozap "github.com/nuclio/zap"
	"github.com/stretchr/testify/suite"
)

type TestSuite struct {
	suite.Suite
	trigger kafka
	logger  logger.Logger
}

func (suite *TestSuite) SetupSuite() {
	suite.logger, _ = nucliozap.NewNuclioZapTest("test")
	suite.trigger = kafka{
		AbstractTrigger: trigger.AbstractTrigger{
			Logger: suite.logger,
		},
		configuration: &Configuration{},
	}
}

func (suite *TestSuite) TestPopulateValuesFromMountedSecrets() {

	// mock a mounted secret by creating a files in a temp dir
	// and setting the configuration fields to point to it

	// create temp dir
	tempDir, err := os.MkdirTemp("", "test")
	suite.Require().NoError(err)
	defer os.RemoveAll(tempDir)

	sensitiveConfigFields := []struct {
		fileName    string
		value       string
		configField *string
	}{
		{"accessKey", "test-access-key", &suite.trigger.configuration.AccessKey},
		{"AccessCert", "test-access-certificate", &suite.trigger.configuration.AccessCertificate},
		{"caCert", "test-ca-certificate", &suite.trigger.configuration.CACert},
		{"SASLPassword", "test-sasl-password", &suite.trigger.configuration.SASL.Password},
		{"SASLClientSecret", "test-sasl-client-secret", &suite.trigger.configuration.SASL.OAuth.ClientSecret},
	}

	for _, field := range sensitiveConfigFields {

		// create files
		filePath := path.Join(tempDir, field.fileName)
		err = os.WriteFile(filePath, []byte(field.value), 0644)
		suite.Require().NoError(err)

		// set config value
		*field.configField = field.fileName
	}

	// set configuration fields to point to the temp dir
	suite.trigger.configuration.SecretPath = tempDir

	err = suite.trigger.configuration.populateValuesFromMountedSecrets(suite.logger)
	suite.Require().NoError(err)

	for _, field := range sensitiveConfigFields {
		suite.Require().Equal(field.value, *field.configField)
	}
}

func TestKafkaSuite(t *testing.T) {
	suite.Run(t, new(TestSuite))
}
