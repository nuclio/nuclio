package common

import (
	"context"
	"regexp"
	"strings"
	"testing"

	"github.com/nuclio/logger"
	nucliozap "github.com/nuclio/zap"
	"github.com/stretchr/testify/suite"
	k8sfake "k8s.io/client-go/kubernetes/fake"
)

type ScrubberTestSuite struct {
	suite.Suite
	logger       logger.Logger
	ctx          context.Context
	k8sClientSet *k8sfake.Clientset
	scrubber     *AbstractScrubber
}

func (suite *ScrubberTestSuite) SetupTest() {
	suite.logger, _ = nucliozap.NewNuclioZapTest("test")
	suite.ctx = context.Background()
	suite.k8sClientSet = k8sfake.NewSimpleClientset()
	suite.scrubber = NewAbstractScrubber(suite.logger, []*regexp.Regexp{}, suite.k8sClientSet, ReferencePrefix, "test", "test", func(name string) bool {
		return false
	})
}

func (suite *ScrubberTestSuite) TestGenerateObjectSecretName() {

	for _, testCase := range []struct {
		name                 string
		objectName           string
		expectedResultPrefix string
	}{
		// Function secret names
		{
			name:                 "FunctionSecret-Sanity",
			objectName:           "my-function",
			expectedResultPrefix: "nuclio-my-function",
		},
		{
			name:                 "FunctionSecret-FunctionNameWithTrailingDashes",
			objectName:           "my-function-_",
			expectedResultPrefix: "nuclio-my-function",
		},
		{
			name:                 "FunctionSecret-LongFunctionName",
			objectName:           "my-function-with-a-very-long-name-which-is-more-than-63-characters-long",
			expectedResultPrefix: "nuclio-my-function-with-a-very-long-name-which-is-more", // nolint: misspell
		},
	} {
		suite.Run(testCase.name, func() {
			secretName := suite.scrubber.generateObjectSecretName(testCase.objectName)
			suite.logger.DebugWith("Generated secret name", "secretName", secretName)
			suite.Require().True(strings.HasPrefix(secretName, testCase.expectedResultPrefix))
		})
	}
}

func TestScrubberTestSuite(t *testing.T) {
	suite.Run(t, new(ScrubberTestSuite))
}
