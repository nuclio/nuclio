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

package prometheus

import (
	"strconv"

	"github.com/nuclio/nuclio/pkg/errors"
	"github.com/nuclio/nuclio/pkg/processor/runtime"
	"github.com/nuclio/nuclio/pkg/processor/trigger"
	"github.com/nuclio/nuclio/pkg/processor/worker"

	"github.com/prometheus/client_golang/prometheus"
)

type WorkerGatherer struct {
	worker                                 *worker.Worker
	prevRuntimeStatistics                  runtime.Statistics
	handledEventsDurationMillisecondsSum   prometheus.Counter
	handledEventsDurationMillisecondsCount prometheus.Counter
}

func NewWorkerGatherer(instanceName string,
	trigger trigger.Trigger,
	worker *worker.Worker,
	metricRegistry *prometheus.Registry) (*WorkerGatherer, error) {

	newWorkerGatherer := &WorkerGatherer{
		worker: worker,
	}

	// base labels for handle events
	labels := prometheus.Labels{
		"instance":      instanceName,
		"trigger_class": trigger.GetClass(),
		"trigger_kind":  trigger.GetKind(),
		"trigger_id":    trigger.GetID(),
		"worker_index":  strconv.Itoa(worker.GetIndex()),
	}

	newWorkerGatherer.handledEventsDurationMillisecondsSum = prometheus.NewCounter(prometheus.CounterOpts{
		Name:        "nuclio_processor_handled_events_duration_milliseconds_sum",
		Help:        "Total sum of milliseconds it took to handle events",
		ConstLabels: labels,
	})

	if err := metricRegistry.Register(newWorkerGatherer.handledEventsDurationMillisecondsSum); err != nil {
		return nil, errors.Wrap(err, "Failed to register handledEventsDurationSum")
	}

	newWorkerGatherer.handledEventsDurationMillisecondsCount = prometheus.NewCounter(prometheus.CounterOpts{
		Name:        "nuclio_processor_handled_events_duration_milliseconds_count",
		Help:        "Number of measurements taken for nuclio_processor_handled_events_duration_sum",
		ConstLabels: labels,
	})

	if err := metricRegistry.Register(newWorkerGatherer.handledEventsDurationMillisecondsCount); err != nil {
		return nil, errors.Wrap(err, "Failed to register handledEventsDurationCount")
	}

	return newWorkerGatherer, nil
}

func (wg *WorkerGatherer) Gather() error {

	// read current stats
	currentRuntimeStatistics := *wg.worker.GetRuntime().GetStatistics()

	// diff from previous to get this period
	diffRuntimeStatistics := currentRuntimeStatistics.DiffFrom(&wg.prevRuntimeStatistics)

	wg.handledEventsDurationMillisecondsSum.Add(float64(diffRuntimeStatistics.DurationMilliSecondsSum))
	wg.handledEventsDurationMillisecondsCount.Add(float64(diffRuntimeStatistics.DurationMilliSecondsCount))

	// save previous
	wg.prevRuntimeStatistics = currentRuntimeStatistics

	return nil
}
