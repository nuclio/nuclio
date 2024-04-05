package platform

import (
	"context"
	"github.com/nuclio/logger"
	"github.com/nuclio/nuclio/pkg/platform/kube/ingress"
	nucliozap "github.com/nuclio/zap"
	"github.com/stretchr/testify/suite"
	k8sfake "k8s.io/client-go/kubernetes/fake"
	"testing"
)

type ScrubberTestSuite struct {
	suite.Suite
	logger       logger.Logger
	ctx          context.Context
	k8sClientSet *k8sfake.Clientset
	scrubber     *APIGatewayScrubber
}

func (suite *ScrubberTestSuite) SetupTest() {
	suite.logger, _ = nucliozap.NewNuclioZapTest("test")
	suite.ctx = context.Background()
	suite.k8sClientSet = k8sfake.NewSimpleClientset()
	suite.scrubber = NewAPIGatewayScrubber(GetAPIGatewaySensitiveField(), suite.k8sClientSet)
}

func (suite *ScrubberTestSuite) TestScrubBasics() {
	apiGatewayConfig := &APIGatewayConfig{Meta: APIGatewayMeta{}, Spec: APIGatewaySpec{
		Host:               "host.com",
		Name:               "test-scrubber",
		AuthenticationMode: ingress.AuthenticationModeBasicAuth,
		Authentication: &APIGatewayAuthenticationSpec{BasicAuth: &BasicAuth{
			Username: "test",
			Password: "my-password",
		}},
	}}

	// scrub the function config
	scrubbedInterface, secretMap, err := suite.scrubber.Scrub(apiGatewayConfig, nil, GetAPIGatewaySensitiveField())
	scrubbedApiGatewayConfig := GetAPIGatewayConfigFromInterface(scrubbedInterface)
	suite.Require().NotEqual(apiGatewayConfig.Spec.Authentication.BasicAuth.Password, scrubbedApiGatewayConfig.Spec.Authentication.BasicAuth.Password)
	suite.Require().NoError(err)

	suite.logger.DebugWith("Scrubbed function config", "functionConfig", scrubbedApiGatewayConfig, "secretMap", secretMap)

	suite.Require().NotEmpty(secretMap)

	restoredInterface, err := suite.scrubber.Restore(scrubbedApiGatewayConfig, secretMap)
	restoredApiGatewayConfig := GetAPIGatewayConfigFromInterface(restoredInterface)
	suite.Require().NoError(err)
	suite.Require().Equal(apiGatewayConfig.Spec.Authentication.BasicAuth.Password, restoredApiGatewayConfig.Spec.Authentication.BasicAuth.Password)

	suite.logger.DebugWith("Restored function config", "functionConfig", restoredApiGatewayConfig)
	suite.Require().Equal(apiGatewayConfig, restoredApiGatewayConfig)
}

func TestScrubberTestSuite(t *testing.T) {
	suite.Run(t, new(ScrubberTestSuite))
}
