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

package test

import (
	"os"
	"testing"

	"github.com/nuclio/nuclio/pkg/platform"
	"github.com/nuclio/nuclio/pkg/platform/factory"

	"github.com/nuclio/logger"
	"github.com/nuclio/zap"
	"github.com/stretchr/testify/suite"
)

type testSuite struct {
	suite.Suite
	logger   logger.Logger
	platform platform.Platform
}

func (suite *testSuite) SetupTest() {
	var err error

	// create logger
	suite.logger, err = nucliozap.NewNuclioZapTest("test")
	suite.Require().NoError(err, "Logger should create successfully")

	platformName := os.Getenv("NUCLIO_PLATFORM")
	if platformName == "" {
		platformName = "local"
	}

	suite.platform, err = factory.CreatePlatform(suite.logger, "kube", nil)
	suite.Require().NoError(err, "Platform should create successfully")
}

//
// Function
//

type functionTestSuite struct {
	testSuite
}

func (suite *functionTestSuite) TestCreateConcurrent() {

}

func TestProjectTestSuite(t *testing.T) {
	if testing.Short() {
		return
	}

	// suite.Run(t, new(functionTestSuite))
}
