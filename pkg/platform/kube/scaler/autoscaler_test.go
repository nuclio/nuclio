package scaler

import (
	"testing"
	"time"

	"github.com/nuclio/nuclio/pkg/functionconfig"

	"github.com/nuclio/logger"
	"github.com/nuclio/zap"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
)

type autoScalerTest struct {
	mock.Mock
	suite.Suite
	logger     logger.Logger
	autoscaler *Autoscaler
	ch         chan metricEntry
}

func (suite *autoScalerTest) scaleFunctionToZero(namespace string, functionName string) {
	suite.Called(namespace, functionName)
}

func (suite *autoScalerTest) SetupTest() {
	var err error
	suite.ch = make(chan metricEntry)
	suite.autoscaler = &Autoscaler{
		logger:         suite.logger,
		metricsChannel: suite.ch,
		metricsMap:     make(functionMetricTypeMap),
		functionScaler: suite,
		metricName:     "fakeSource",
	}
	suite.Require().NoError(err)
	suite.On("scaleFunctionToZero", mock.Anything, mock.Anything).Return()
	suite.Calls = []mock.Call{}
	suite.autoscaler.windowSize = time.Duration(1 * time.Minute)
}

func (suite *autoScalerTest) SetupSuite() {
	suite.logger, _ = nucliozap.NewNuclioZapTest("test")
}

func (suite *autoScalerTest) TestScaleToZero() {
	t, _ := time.ParseDuration("2m")

	suite.autoscaler.addMetricEntry("f", "fakeSource", metricEntry{
		timestamp:    time.Now().Add(-t),
		value:        0,
		functionName: "f",
		metricName:   "fakeSource",
	})

	suite.autoscaler.checkFunctionsToScale(time.Now(), map[string]*functionconfig.ConfigWithStatus{
		"f": suite.getFunctionWithStatus(functionconfig.FunctionStateReady),
	})

	suite.AssertNumberOfCalls(suite.T(), "scaleFunctionToZero", 1)
}

func (suite *autoScalerTest) TestNotScale() {
	t, _ := time.ParseDuration("5m")
	suite.autoscaler.windowSize = t

	for _, duration := range []string{"4m", "200s", "3m", "2m", "100s"} {
		suite.addEntry("f", duration, 0)
	}

	suite.autoscaler.checkFunctionsToScale(time.Now(), map[string]*functionconfig.ConfigWithStatus{
		"f": suite.getFunctionWithStatus(functionconfig.FunctionStateReady),
	})
	suite.AssertNumberOfCalls(suite.T(), "scaleFunctionToZero", 0)

	for _, duration := range []string{"50s", "40s", "30s", "20s", "10s"} {
		suite.addEntry("f", duration, 0)
	}
	suite.addEntry("f", "5s", 9)

	suite.autoscaler.checkFunctionsToScale(time.Now(), map[string]*functionconfig.ConfigWithStatus{
		"f": suite.getFunctionWithStatus(functionconfig.FunctionStateReady),
	})
	suite.AssertNumberOfCalls(suite.T(), "scaleFunctionToZero", 0)
}

func (suite *autoScalerTest) TestScaleToZeroWithNoEvents() {
	suite.autoscaler.checkFunctionsToScale(time.Now(), map[string]*functionconfig.ConfigWithStatus{
		"f": suite.getFunctionWithStatus(functionconfig.FunctionStateReady),
	})
	suite.AssertNumberOfCalls(suite.T(), "scaleFunctionToZero", 0)
}

func (suite *autoScalerTest) TestScaleToZeroMultipleFunctions() {
	t1, _ := time.ParseDuration("2m")
	t2, _ := time.ParseDuration("30s")

	suite.autoscaler.addMetricEntry("foo", "fakeSource", metricEntry{
		timestamp:    time.Now().Add(-t1),
		value:        0,
		functionName: "foo",
		metricName:   "fakeSource",
	})

	suite.autoscaler.addMetricEntry("bar", "fakeSource", metricEntry{
		timestamp:    time.Now().Add(-t2),
		value:        0,
		functionName: "bar",
		metricName:   "fakeSource",
	})

	suite.autoscaler.checkFunctionsToScale(time.Now().Add(t1), map[string]*functionconfig.ConfigWithStatus{
		"f":   suite.getFunctionWithStatus(functionconfig.FunctionStateReady),
		"bar": suite.getFunctionWithStatus(functionconfig.FunctionStateReady),
		"tar": suite.getFunctionWithStatus(functionconfig.FunctionStateReady),
	})

	suite.AssertNumberOfCalls(suite.T(), "scaleFunctionToZero", 1)
}

func (suite *autoScalerTest) TestScaleToZeroMultipleTimes() {
	t1, _ := time.ParseDuration("2m")

	suite.autoscaler.addMetricEntry("f", "fakeSource", metricEntry{
		timestamp:    time.Now().Add(-t1),
		value:        0,
		functionName: "f",
		metricName:   "fakeSource",
	})

	suite.autoscaler.checkFunctionsToScale(time.Now().Add(t1), map[string]*functionconfig.ConfigWithStatus{
		"f": suite.getFunctionWithStatus(functionconfig.FunctionStateReady),
	})

	suite.AssertNumberOfCalls(suite.T(), "scaleFunctionToZero", 1)
	suite.Require().Equal(len(suite.autoscaler.metricsMap["f"]), 0)

	t2, _ := time.ParseDuration("30s")
	suite.autoscaler.addMetricEntry("f", "fakeSource", metricEntry{
		timestamp:    time.Now().Add(-t2),
		value:        0,
		functionName: "f",
		metricName:   "fakeSource",
	})

	suite.autoscaler.checkFunctionsToScale(time.Now(), map[string]*functionconfig.ConfigWithStatus{
		"f": suite.getFunctionWithStatus(functionconfig.FunctionStateScaledToZero),
	})

	suite.AssertNumberOfCalls(suite.T(), "scaleFunctionToZero", 1)

	suite.autoscaler.addMetricEntry("f", "fakeSource", metricEntry{
		timestamp:    time.Now(),
		value:        0,
		functionName: "f",
		metricName:   "fakeSource",
	})

	suite.autoscaler.checkFunctionsToScale(time.Now().Add(t1), map[string]*functionconfig.ConfigWithStatus{
		"f": suite.getFunctionWithStatus(functionconfig.FunctionStateScaledToZero),
	})

	suite.AssertNumberOfCalls(suite.T(), "scaleFunctionToZero", 1)
	suite.Require().Equal(len(suite.autoscaler.metricsMap["f"]), 0)

	// simulate the return of the function 30 mins later with ready state
	t3, _ := time.ParseDuration("30m")
	suite.autoscaler.checkFunctionsToScale(time.Now().Add(t3), map[string]*functionconfig.ConfigWithStatus{
		"f": suite.getFunctionWithStatus(functionconfig.FunctionStateReady),
	})

	suite.AssertNumberOfCalls(suite.T(), "scaleFunctionToZero", 1)
	suite.Require().Equal(len(suite.autoscaler.metricsMap["f"]), 0)
}

func (suite *autoScalerTest) addEntry(key string, duration string, value int64) {
	t, _ := time.ParseDuration(duration)
	suite.autoscaler.addMetricEntry(key, "fakeSource", metricEntry{
		timestamp:    time.Now().Add(-t),
		value:        value,
		functionName: "bb",
		metricName:   "fakeSource",
	})
}

func (suite *autoScalerTest) getFunctionWithStatus(state functionconfig.FunctionState) *functionconfig.ConfigWithStatus {
	return &functionconfig.ConfigWithStatus{
		Status: functionconfig.Status{
			State: state,
		},
	}
}

func TestAutoscale(t *testing.T) {
	suite.Run(t, new(autoScalerTest))
}
