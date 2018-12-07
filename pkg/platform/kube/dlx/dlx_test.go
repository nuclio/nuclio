package dlx

import (
	"fmt"
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/nuclio/logger"
	"github.com/nuclio/zap"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
)

type dlxTest struct {
	suite.Suite
	logger          logger.Logger
	functionStarter *FunctionStarter
	mockNuclio      *mockNuclioWrapper
}

type mockNuclioWrapper struct {
	mock.Mock
}

func (mnw *mockNuclioWrapper) updateFunctionStatus(functionName string) {
	mnw.Called(functionName)
}

func (mnw *mockNuclioWrapper) waitFunctionReadiness(functionName string, ch chan bool) {
	mnw.Called(functionName, ch)
	toFail := mnw.TestData().Get("toFail")

	if !toFail.Bool(false) {
		ch <- true
	}
}

func (suite *dlxTest) SetupSuite() {
	var err error
	suite.logger, _ = nucliozap.NewNuclioZapTest("test")

	suite.functionStarter = &FunctionStarter{
		logger:                   suite.logger,
		nuclioActioner:           suite.mockNuclio,
		functionSinksMap:         make(functionSinksMap),
		namespace:                "b",
		functionReadinnesTimeout: time.Duration(1 * time.Second),
	}
	suite.Require().NoError(err)
}

func (suite *dlxTest) TestDlxMultipleRequests() {
	suite.mockNuclio = new(mockNuclioWrapper)
	suite.functionStarter.nuclioActioner = suite.mockNuclio
	wg := sync.WaitGroup{}
	suite.mockNuclio.On("updateFunctionStatus", mock.Anything).Return()
	suite.mockNuclio.On("waitFunctionReadiness", mock.Anything, mock.Anything).Return()

	for i := 0; i < 200; i++ {
		wg.Add(1)
		go func() {
			ch := make(responseChannel)
			suite.functionStarter.HandleFunctionStart(fmt.Sprintf("test%d", i), ch)
			r := <-ch
			suite.logger.DebugWith("Got response", "r", r)
			wg.Done()
			suite.Require().Equal(http.StatusOK, r.Status)
		}()
	}
	wg.Wait()
}

func (suite *dlxTest) TestDlxMultipleRequestsSameTarget() {
	suite.mockNuclio = new(mockNuclioWrapper)
	suite.functionStarter.nuclioActioner = suite.mockNuclio
	wg := sync.WaitGroup{}
	suite.mockNuclio.On("updateFunctionStatus", mock.Anything).Return()
	suite.mockNuclio.On("waitFunctionReadiness", mock.Anything, mock.Anything).Return()

	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func() {
			ch := make(responseChannel)
			suite.functionStarter.HandleFunctionStart("test", ch)
			r := <-ch
			suite.logger.DebugWith("Got response", "r", r)
			wg.Done()
			suite.Require().Equal(http.StatusOK, r.Status)
		}()
	}

	wg.Wait()
	suite.Require().True(suite.mockNuclio.AssertNumberOfCalls(suite.T(), "updateFunctionStatus", 1))
}

func (suite *dlxTest) TestDlxRequestFailure() {
	suite.mockNuclio = new(mockNuclioWrapper)
	suite.functionStarter.nuclioActioner = suite.mockNuclio
	suite.mockNuclio.TestData().Set("toFail", true)
	suite.mockNuclio.On("updateFunctionStatus", mock.Anything).Return()
	suite.mockNuclio.On("waitFunctionReadiness", mock.Anything, mock.Anything).Return()

	ch := make(responseChannel)
	suite.functionStarter.HandleFunctionStart("test", ch)
	r := <-ch
	suite.logger.DebugWith("Got response", "r", r)
	suite.Require().Equal(http.StatusGatewayTimeout, r.Status)
}

func TestAutoscale(t *testing.T) {
	suite.Run(t, new(dlxTest))
}
