package dlx

import (
	"github.com/nuclio/logger"
	"github.com/nuclio/zap"
	"github.com/stretchr/testify/suite"
	"github.com/valyala/fasthttp"
	"testing"
)

type dlxTest struct {
	suite.Suite
	logger logger.Logger
	functionStarter *FunctionStarter
	testServer     fasthttp.Server
}

func (suite *dlxTest) SetupSuite() {
	var err error
	suite.logger, _ = nucliozap.NewNuclioZapTest("test")
	//functionStarter, err := NewFunctionStarter(suite.logger, k8s_fake.NewSimpleClientset())
	//suite.functionStarter = functionStarter
	suite.Require().NoError(err)
}


func TestAutoscale(t *testing.T) {
	suite.Run(t, new(dlxTest))
}