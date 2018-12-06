package scaler

import (
	"github.com/nuclio/logger"
	"github.com/stretchr/testify/mock"

	"github.com/nuclio/nuclio/pkg/functionconfig"
	"github.com/nuclio/zap"
	"github.com/stretchr/testify/suite"
	"testing"
	"time"
)

type autoScalerTest struct {
	mock.Mock
	suite.Suite
	logger     logger.Logger
	autoscaler *Autoscale
	ch         chan metricEntry
}

func (suite *autoScalerTest) ScaleToZero(namespace string, functionName string) {
	suite.Called(namespace, functionName)
}

func (suite *autoScalerTest) SetupSuite() {
	var err error
	suite.logger, _ = nucliozap.NewNuclioZapTest("test")
	suite.ch = make(chan metricEntry)
	suite.autoscaler = &Autoscale{
		logger:         suite.logger,
		metricsChannel: suite.ch,
		metricsMap:     make(functionMetricTypeMap),
		scalerFunction: suite.ScaleToZero,
		metricType:     "fakeSource",
	}
	suite.Require().NoError(err)
	suite.On("ScaleToZero", mock.Anything, mock.Anything).Return()
}

func (suite *autoScalerTest) TestScaleToZero() {
	t, _ := time.ParseDuration("2m")
	suite.autoscaler.windowSize = time.Duration(1 * time.Minute)

	suite.autoscaler.AddMetricEntry("f", "fakeSource", metricEntry{
		timestamp:    time.Now().Add(-t),
		value:        0,
		functionName: "bb",
		metricType:   "fakeSource",
	})

	suite.autoscaler.CheckFunctionsToScale(time.Now(), map[string]*functionconfig.Spec{
		"f": nil,
	})

	suite.AssertNumberOfCalls(suite.T(), "ScaleToZero", 1)
}

func (suite *autoScalerTest) TestNotScale() {
	t, _ := time.ParseDuration("5m")
	suite.autoscaler.windowSize = t

	for _, duration := range []string{"4m", "200s", "3m", "2m", "100s"} {
		suite.addEntry("f", duration, 0)
	}

	suite.autoscaler.CheckFunctionsToScale(time.Now(), map[string]*functionconfig.Spec{
		"f": nil,
	})
	suite.AssertNumberOfCalls(suite.T(), "ScaleToZero", 0)

	for _, duration := range []string{"50s", "40s", "30s", "20s", "10s"} {
		suite.addEntry("f", duration, 0)
	}
	suite.addEntry("f", "5s", 9)

	suite.autoscaler.CheckFunctionsToScale(time.Now(), map[string]*functionconfig.Spec{
		"f": nil,
	})
	suite.AssertNumberOfCalls(suite.T(), "ScaleToZero", 0)
}

func (suite *autoScalerTest) TestScaleToZeroMultipleFunctions() {

}

func (suite *autoScalerTest) addEntry(key string, duration string, value int64) {
	t, _ := time.ParseDuration(duration)
	suite.autoscaler.AddMetricEntry(key, "fakeSource", metricEntry{
		timestamp:    time.Now().Add(-t),
		value:        value,
		functionName: "bb",
		metricType:   "fakeSource",
	})
}

func TestAutoscale(t *testing.T) {
	suite.Run(t, new(autoScalerTest))
}
