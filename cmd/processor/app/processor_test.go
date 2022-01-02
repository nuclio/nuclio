//go:build test_unit

/*
Copyright 2017 The Nuclio Authors.

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

package app

import (
	"fmt"
	"testing"

	"github.com/nuclio/nuclio/pkg/functionconfig"
	"github.com/nuclio/nuclio/pkg/platformconfig"
	"github.com/nuclio/nuclio/pkg/processor"
	// load cron trigger for tests purposes
	_ "github.com/nuclio/nuclio/pkg/processor/trigger/cron"
	"github.com/nuclio/nuclio/pkg/processor/worker"

	"github.com/nuclio/logger"
	nucliozap "github.com/nuclio/zap"
	"github.com/stretchr/testify/suite"
)

type TriggerTestSuite struct {
	suite.Suite
	logger logger.Logger
}

func (suite *TriggerTestSuite) SetupSuite() {
	var err error
	suite.logger, err = nucliozap.NewNuclioZapTest("test")
	suite.Require().NoError(err)
}

func (suite *TriggerTestSuite) TestCreateManyTriggersWithSameWorkerAllocatorName() {
	processorInstance := Processor{
		logger:                suite.logger,
		functionLogger:        suite.logger.GetChild("some-function-logger"),
		namedWorkerAllocators: worker.NewAllocatorSyncMap(),
	}
	totalTriggers := 1000
	triggerSpecs := map[string]functionconfig.Trigger{}
	for i := 0; i < totalTriggers; i++ {
		triggerName := fmt.Sprintf("Cron%d", i)
		triggerSpecs[triggerName] = functionconfig.Trigger{
			Name:                triggerName,
			Kind:                "cron",
			WorkerAllocatorName: "sameAllocator",
			Attributes: map[string]interface{}{
				"interval": "24h",
			},
		}
	}
	triggers, err := processorInstance.createTriggers(&processor.Configuration{
		Config: functionconfig.Config{
			Spec: functionconfig.Spec{
				Runtime:  "golang",
				Handler:  "nuclio:builtin",
				Triggers: triggerSpecs,
			},
		},
		PlatformConfig: &platformconfig.Config{
			Kind: "local",
		},
	})

	suite.Require().NoError(err)
	suite.Require().Len(triggers, totalTriggers)
	suite.Require().Len(processorInstance.namedWorkerAllocators.Keys(),
		1,
		"Expected only one named allocator to be created")
}

func TestTriggerTestSuite(t *testing.T) {
	suite.Run(t, new(TriggerTestSuite))
}
