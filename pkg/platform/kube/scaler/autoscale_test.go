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
	ch         chan entry
}

type FakeScaler struct {

}

func (fs *FakeScaler) Scale(namespace string, functionName string, target int) {

}

func (suite *autoScalerTest) SetupSuite() {
	suite.logger, _ = nucliozap.NewNuclioZapTest("test")
	suite.ch = make(chan entry)
	suite.autoscaler = NewAutoScaler(suite.logger, "default", suite.ch, new(FakeScaler))
}

func (suite *autoScalerTest) TestScaleToZero() {
	fkey := statKey{namespace: "bla", functionName: "b", sourceType: "fakeSource"}
	t, _ := time.ParseDuration("2m")
	suite.autoscaler.addEntry(fkey, entry{
		timestamp: time.Now().Add(-t),
		value: 1,
		namespace: "bla",
		functionName: "b",
	})

	suite.autoscaler.CheckToScale(time.Now(), map[statKey]*functionconfig.Spec{
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
	fkey := statKey{namespace: "bla", functionName: "b", sourceType: "fakeSource"}

	for _, duration := range []string{"4m", "200s", "3m", "2m", "100s"} {
		t, _ := time.ParseDuration(duration)
		suite.autoscaler.addEntry(fkey, entry{
			timestamp: time.Now().Add(-t),
			value: 1,
			namespace: "bla",
			functionName: "b",
		})
	}

	suite.autoscaler.CheckToScale(time.Now(), map[statKey]*functionconfig.Spec{
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
		suite.autoscaler.addEntry(fkey, entry{
			timestamp: time.Now().Add(-t),
			value: 1,
			namespace: "bla",
			functionName: "b",
		})
	}

	suite.autoscaler.addEntry(fkey, entry{
		timestamp: time.Now(),
		value: 9,
		namespace: "bla",
		functionName: "b",
	})

	addDuration, _ := time.ParseDuration("3m")
	suite.autoscaler.CheckToScale(time.Now().Add(addDuration), map[statKey]*functionconfig.Spec{
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