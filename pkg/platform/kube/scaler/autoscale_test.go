package scaler

import (
	"github.com/nuclio/logger"

	"github.com/nuclio/nuclio/pkg/functionconfig"
	"github.com/nuclio/zap"
	"github.com/stretchr/testify/suite"
	"testing"
	"time"
)

type autoScalerTest struct {
	suite.Suite
	logger logger.Logger
	autoscaler *Autoscale
	ch         chan metricEntry
}

func (suite *autoScalerTest) ScaleToZero(namespace string, functionName string, target int) {

}

func (suite *autoScalerTest) SetupSuite() {
	var err error
	suite.logger, _ = nucliozap.NewNuclioZapTest("test")
	suite.ch = make(chan metricEntry)
	suite.autoscaler, err = NewAutoScaler(suite.logger,"default", nil, suite.ch)
	suite.Require().NoError(err)
}

func (suite *autoScalerTest) TestScaleToZero() {
	fkey := functionMetricKey{namespace: "bla", functionName: "b", sourceType: "fakeSource"}
	t, _ := time.ParseDuration("2m")
	suite.autoscaler.AddMetricEntry(fkey, metricEntry{
		timestamp: time.Now().Add(-t),
		value: 1,
		functionMetricKey: functionMetricKey{
			namespace: "bla",
			functionName: "bb",
			sourceType: "fakeSource",
		},
	})

	suite.autoscaler.CheckFunctionsToScale(time.Now(), map[functionMetricKey]*functionconfig.Spec{
		fkey: {
			Metrics: []functionconfig.Metric{
				{
					SourceType: "fakeSource",
					WindowSize: "1m",
					ThresholdValue: 5,
				},
			},
		},
	})
}

func (suite *autoScalerTest) TestNotScale() {
	fkey := functionMetricKey{namespace: "bla", functionName: "b", sourceType: "fakeSource"}

	for _, duration := range []string{"4m", "200s", "3m", "2m", "100s"} {
		t, _ := time.ParseDuration(duration)
		suite.autoscaler.AddMetricEntry(fkey, metricEntry{
			timestamp: time.Now().Add(-t),
			value: 1,
			functionMetricKey: functionMetricKey{
				namespace: "bla",
				functionName: "bb",
				sourceType: "fakeSource",
			},
		})
	}

	suite.autoscaler.CheckFunctionsToScale(time.Now(), map[functionMetricKey]*functionconfig.Spec{
		fkey: {
			Metrics: []functionconfig.Metric{
				{
					SourceType: "fakeSource",
					WindowSize: "5m",
					ThresholdValue: 5,
				},
			},
		},
	})

	for _, duration := range []string{"50s", "40s", "30s", "20s", "10s"} {
		t, _ := time.ParseDuration(duration)
		suite.autoscaler.AddMetricEntry(fkey, metricEntry{
			timestamp: time.Now().Add(-t),
			value: 1,
			functionMetricKey: functionMetricKey{
				namespace: "bla",
				functionName: "bb",
				sourceType: "fakeSource",
			},
		})
	}

	suite.autoscaler.AddMetricEntry(fkey, metricEntry{
		timestamp: time.Now(),
		value: 9,
		functionMetricKey: functionMetricKey{
			namespace: "bla",
			functionName: "bb",
			sourceType: "fakeSource",
		},
	})

	addDuration, _ := time.ParseDuration("3m")
	suite.autoscaler.CheckFunctionsToScale(time.Now().Add(addDuration), map[functionMetricKey]*functionconfig.Spec{
		fkey: {
			Metrics: []functionconfig.Metric{
				{
					SourceType: "fakeSource",
					WindowSize: "5m",
					ThresholdValue: 5,
				},
			},
		},
	})
}

func TestAutoscale(t *testing.T) {
	suite.Run(t, new(autoScalerTest))
}