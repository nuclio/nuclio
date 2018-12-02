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
	fkey := statKey{namespace: "bla", functionName: "b"}
	suite.autoscaler.addEntry(fkey, entry{
		timestamp: time.Now(),
		value: 1,
		namespace: "bla",
		functionName: "b",
		sourceType: "fakeSource",
	})

	suite.autoscaler.CheckToScale(time.Now(), map[statKey]*functionconfig.Spec{
		fkey: {
			Metrics: []functionconfig.Metric{
				{
					SourceType: "fakeSource",
					WindowSize: time.Duration(1*time.Hour),
				},
			},
		},
	})
}

func TestAutoscale(t *testing.T) {
	suite.Run(t, new(autoScalerTest))
}