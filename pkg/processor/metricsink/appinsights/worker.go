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
	"strconv"

	"github.com/nuclio/nuclio/pkg/processor/runtime"
	"github.com/nuclio/nuclio/pkg/processor/trigger"
	"github.com/nuclio/nuclio/pkg/processor/worker"

	"github.com/Microsoft/ApplicationInsights-Go/appinsights"
)

type WorkerGatherer struct {
	worker                *worker.Worker
	prevRuntimeStatistics runtime.Statistics
	client                appinsights.TelemetryClient
}

func newWorkerGatherer(trigger trigger.Trigger,
	worker *worker.Worker,
	client appinsights.TelemetryClient) (*WorkerGatherer, error) {

	newWorkerGatherer := &WorkerGatherer{
		worker: worker,
		client: client,
	}

	return newWorkerGatherer, nil
}

func (wg *WorkerGatherer) Gather() error {

	// read current stats
	currentRuntimeStatistics := *wg.worker.GetRuntime().GetStatistics()

	// diff from previous to get this period
	diffRuntimeStatistics := currentRuntimeStatistics.DiffFrom(&wg.prevRuntimeStatistics)

	// save previous
	wg.prevRuntimeStatistics = currentRuntimeStatistics

	aggregate := appinsights.NewAggregateMetricTelemetry("FunctionDuration")
	aggregate.Value = float64(diffRuntimeStatistics.DurationMilliSecondsSum)
	aggregate.Count = int(diffRuntimeStatistics.DurationMilliSecondsCount)
	aggregate.Properties["WorkerIndex"] = strconv.Itoa(wg.worker.GetIndex())
	wg.client.Track(aggregate)

	return nil
}
