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

package appinsights

import (
	"github.com/nuclio/nuclio/pkg/processor/trigger"
)

type TriggerGatherer struct {
	trigger        trigger.Trigger
	prevStatistics trigger.Statistics
}

func newTriggerGatherer(trigger trigger.Trigger) (*TriggerGatherer, error) {

	newTriggerGatherer := &TriggerGatherer{
		trigger: trigger,
	}

	return newTriggerGatherer, nil
}

func (esg *TriggerGatherer) Gather() error {

	// read current stats
	currentStatistics := *esg.trigger.GetStatistics()

	// diff from previous to get this period
	// diffStatistics := currentStatistics.DiffFrom(&esg.prevStatistics)

	esg.prevStatistics = currentStatistics

	return nil
}
