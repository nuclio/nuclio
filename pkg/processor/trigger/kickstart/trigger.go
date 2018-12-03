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

package kickstart

import (
	"time"

	"github.com/nuclio/nuclio/pkg/common"
	"github.com/nuclio/nuclio/pkg/functionconfig"
	"github.com/nuclio/nuclio/pkg/processor/trigger"
	"github.com/nuclio/nuclio/pkg/processor/worker"

	"github.com/nuclio/logger"
)

type kickstart struct {
	trigger.AbstractTrigger
	configuration *Configuration
}

func newTrigger(logger logger.Logger,
	workerAllocator worker.Allocator,
	configuration *Configuration) (trigger.Trigger, error) {

	newTrigger := kickstart{
		AbstractTrigger: trigger.AbstractTrigger{
			ID:              configuration.ID,
			Logger:          logger,
			WorkerAllocator: workerAllocator,
			Class:           "async",
			Kind:            "kickstart",
		},
		configuration: configuration,
	}

	newTrigger.Namespace = newTrigger.configuration.RuntimeConfiguration.Meta.Namespace
	newTrigger.FunctionName = newTrigger.configuration.RuntimeConfiguration.Meta.Name
	return &newTrigger, nil
}

func (k *kickstart) Start(checkpoint functionconfig.Checkpoint) error {
	k.Logger.DebugWith("Kickstarting")

	k.AllocateWorkerAndSubmitEvent( // nolint: errcheck
		&k.configuration.Event,
		k.Logger,
		10*time.Second)

	return nil
}

func (k *kickstart) Stop(force bool) (functionconfig.Checkpoint, error) {
	return nil, nil
}

func (k *kickstart) GetConfig() map[string]interface{} {
	return common.StructureToMap(k.configuration)
}
