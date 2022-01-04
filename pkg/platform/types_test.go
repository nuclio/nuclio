//go:build test_unit

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
