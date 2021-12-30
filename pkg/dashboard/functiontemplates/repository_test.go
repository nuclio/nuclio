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

package functiontemplates

import (
	"encoding/base64"
	"testing"

	"github.com/nuclio/nuclio/pkg/functionconfig"

	"github.com/nuclio/logger"
	"github.com/nuclio/zap"
	"github.com/stretchr/testify/suite"
)

type repositoryTestSuite struct {
	suite.Suite
	logger logger.Logger
}

func (suite *repositoryTestSuite) SetupSuite() {
	suite.logger, _ = nucliozap.NewNuclioZapTest("test")
}

func (suite *repositoryTestSuite) TestFilteredGet() {
	functionTemplates := []*generatedFunctionTemplate{
		{
			Name: "template1",
			Configuration: functionconfig.Config{
				Spec: functionconfig.Spec{
					Description: "Template 1 description",
					Runtime:     "template-1-runtime",
				},
			},
			SourceCode: "template 1 source code",
		},
		{
			Name: "template2",
			Configuration: functionconfig.Config{
				Spec: functionconfig.Spec{
					Description: "Template 2 description",
					Runtime:     "template-2-runtime",
				},
			},
			SourceCode: "template 2 source code",
		},
	}

	// create Fetcher
	fetcher, err := NewGeneratedFunctionTemplateFetcher(suite.logger)
	suite.Require().NoError(err, "Failed to create fetcher")

	err = fetcher.SetGeneratedFunctionTemplates(functionTemplates)
	suite.Require().NoError(err, "Failed to set fetcher's templates")

	// create repository
	repository, err := NewRepository(suite.logger, []FunctionTemplateFetcher{fetcher})
	suite.Require().NoError(err, "Failed to create repository")

	// get all templates (nil filter)
	matchedFunctionTemplates := repository.GetFunctionTemplates(nil)
	suite.Require().Len(matchedFunctionTemplates, 2)

	// verify that returned value holds function source
	for functionTemplatesIdx, functionTemplate := range functionTemplates {
		decodedSourceCode, err := base64.StdEncoding.DecodeString(matchedFunctionTemplates[functionTemplatesIdx].FunctionConfig.Spec.Build.FunctionSourceCode)
		suite.Require().NoError(err, "Failed to decode function source")
		suite.Require().Equal(functionTemplate.SourceCode, string(decodedSourceCode))
	}

	// get, filtered by name. expect template2
	matchedFunctionTemplates = repository.GetFunctionTemplates(&Filter{"template2"})
	suite.Require().Len(matchedFunctionTemplates, 1)
	suite.Require().Equal("template2", matchedFunctionTemplates[0].Name)

	// get, filtered by configuration. expect template1
	matchedFunctionTemplates = repository.GetFunctionTemplates(&Filter{"template-1-"})
	suite.Require().Len(matchedFunctionTemplates, 1)
	suite.Require().Equal("template1", matchedFunctionTemplates[0].Name)
}

func TestRepository(t *testing.T) {
	suite.Run(t, new(repositoryTestSuite))
}
