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

package basicmqtt

import (
	"github.com/nuclio/nuclio/pkg/processor/trigger"
	"github.com/nuclio/nuclio/pkg/processor/trigger/mqtt"
	"github.com/nuclio/nuclio/pkg/processor/worker"

	"github.com/nuclio/errors"
	"github.com/nuclio/logger"
)

type basicmqtt struct {
	*mqtt.AbstractTrigger
}

func newTrigger(parentLogger logger.Logger,
	workerAllocator worker.Allocator,
	configuration *mqtt.Configuration,
	restartTriggerChan chan trigger.Trigger) (trigger.Trigger, error) {

	newAbstractTrigger, err := mqtt.NewAbstractTrigger(parentLogger.GetChild("mqtt"),
		workerAllocator,
		configuration,
		restartTriggerChan)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to create abstract trigger")
	}

	newTrigger := basicmqtt{
		AbstractTrigger: newAbstractTrigger,
	}
	newTrigger.AbstractTrigger.Trigger = &newTrigger

	return newTrigger, nil
}
