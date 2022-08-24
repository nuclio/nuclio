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

package platform

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

func (suite *TypesTestSuite) TestProjectConfigIsEqual() {
	for _, testCase := range []struct {
		name                string
		firstProjectConfig  ProjectConfig
		secondProjectConfig ProjectConfig
		shouldBeEqual       bool
	}{
		{
			name: "SimpleEqual",
			firstProjectConfig: ProjectConfig{
				Meta: ProjectMeta{
					Name:        "simple-name",
					Namespace:   "simple-namespace",
					Labels:      map[string]string{"label1": "value1"},
					Annotations: map[string]string{"annotation1": "value1"},
				},
				Spec: ProjectSpec{
					Description: "description",
				},
			},
			secondProjectConfig: ProjectConfig{
				Meta: ProjectMeta{
					Name:        "simple-name",
					Namespace:   "simple-namespace",
					Labels:      map[string]string{"label1": "value1"},
					Annotations: map[string]string{"annotation1": "value1"},
				},
				Spec: ProjectSpec{
					Description: "description",
				},
			},
			shouldBeEqual: true,
		},
		{
			name: "EqualWithEmptyAndUndefinedLabelsAndAnnotations",
			firstProjectConfig: ProjectConfig{
				Meta: ProjectMeta{
					Name:        "simple-name",
					Namespace:   "simple-namespace",
					Labels:      map[string]string{},
					Annotations: map[string]string{},
				},
				Spec: ProjectSpec{
					Description: "description",
				},
			},
			secondProjectConfig: ProjectConfig{
				Meta: ProjectMeta{
					Name:      "simple-name",
					Namespace: "simple-namespace",
				},
				Spec: ProjectSpec{
					Description: "description",
				},
			},
			shouldBeEqual: true,
		},
	} {
		suite.Run(testCase.name, func() {
			isEqual := testCase.firstProjectConfig.IsEqual(&testCase.secondProjectConfig, false)
			suite.Require().Equal(testCase.shouldBeEqual, isEqual)
		})
	}
}

func TestTypesTestSuite(t *testing.T) {
	suite.Run(t, new(TypesTestSuite))
}
