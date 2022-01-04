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
	"testing"

	"github.com/nuclio/logger"
	"github.com/nuclio/zap"
	"github.com/stretchr/testify/suite"
)

type TypesTestSuite struct {
	suite.Suite
	logger logger.Logger
}

func (suite *TypesTestSuite) SetupTest() {
	suite.logger, _ = nucliozap.NewNuclioZapTest("test")
}

func (suite *ReaderTestSuite) TestFunctionMetaSkipDeployAnnotationTrue() {
	for _, testCase := range []struct {
		Annotations    map[string]string
		ExpectedResult bool
	}{
		{
			Annotations: map[string]string{
				FunctionAnnotationSkipDeploy: "true",
			},
			ExpectedResult: true,
		},
		{
			Annotations: map[string]string{
				FunctionAnnotationSkipDeploy: "false",
			},
			ExpectedResult: false,
		},
		{
			Annotations:    map[string]string{},
			ExpectedResult: false,
		},
	} {
		functionMeta := Meta{
			Annotations: testCase.Annotations,
		}
		suite.Assert().Equal(testCase.ExpectedResult, ShouldSkipDeploy(functionMeta.Annotations))
	}
}

func (suite *ReaderTestSuite) TestFunctionMetaSkipBuildAnnotationTrue() {
	for _, testCase := range []struct {
		Annotations    map[string]string
		ExpectedResult bool
	}{
		{
			Annotations: map[string]string{
				FunctionAnnotationSkipBuild: "true",
			},
			ExpectedResult: true,
		},
		{
			Annotations: map[string]string{
				FunctionAnnotationSkipBuild: "false",
			},
			ExpectedResult: false,
		},
		{
			Annotations:    map[string]string{},
			ExpectedResult: false,
		},
	} {
		functionMeta := Meta{
			Annotations: testCase.Annotations,
		}
		suite.Assert().Equal(testCase.ExpectedResult, ShouldSkipBuild(functionMeta.Annotations))
	}
}

func (suite *ReaderTestSuite) TestGetInvocationURLs() {
	functionStatus := Status{
		InternalInvocationURLs: []string{"a", "b"},
		ExternalInvocationURLs: []string{"c", "d"},
	}
	suite.Require().Equal([]string{"a", "b", "c", "d"}, functionStatus.InvocationURLs())
}

func TestTypesTestSuite(t *testing.T) {
	suite.Run(t, new(TypesTestSuite))
}
