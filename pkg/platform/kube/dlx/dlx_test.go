package dlx

import (
	"github.com/nuclio/logger"
	"github.com/valyala/fasthttp"
	"github.com/valyala/fasthttp/fake"
	"k8s.io/client-go/kubernetes/fake"

	"github.com/nuclio/zap"
	"github.com/stretchr/testify/suite"
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
	functionStarter, err := NewFunctionStarter(suite.logger, fake.NewSimpleClientset())
	suite.functionStarter = functionStarter
	suite.Require().NoError(err)
}

func (suite *dlxTest) TestScaleToZero() {

	handler := func (ctx *fasthttp.RequestCtx) {
		suite.functionStarter.SendRequestGetResponse()
	}

	if err := fasthttp.ListenAndServe("", handler); err != nil {
		suite.logger.WarnWith("error in ListenAndServe", "err", err)
	}
}

func TestAutoscale(t *testing.T) {
	suite.Run(t, new(dlxTest))
}