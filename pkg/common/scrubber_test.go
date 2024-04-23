//go:build test_unit

/*
Copyright 2024 The Nuclio Authors.

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

package common

import (
	"context"
	v1 "k8s.io/api/core/v1"
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
	suite.scrubber = NewAbstractScrubber(suite.logger, []*regexp.Regexp{}, suite.k8sClientSet, ReferencePrefix, "test", "test", func(secret v1.Secret) bool {
		return false
	})
}

func (suite *ScrubberTestSuite) TestGenerateObjectSecretName() {

	for _, testCase := range []struct {
		name                 string
		objectName           string
		expectedResultPrefix string
	}{
		{
			name:                 "ObjectSecret-Sanity",
			objectName:           "my-object",
			expectedResultPrefix: "nuclio-my-object",
		},
		{
			name:                 "ObjectSecret-ObjectNameWithTrailingDashes",
			objectName:           "my-object-_",
			expectedResultPrefix: "nuclio-my-object",
		},
		{
			name:                 "ObjectSecret-LongObjectName",
			objectName:           "my-object-with-a-very-long-name-which-is-more-than-63-characters-long",
			expectedResultPrefix: "nuclio-my-object-with-a-very-long-name-which-is-more", // nolint: misspell
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
