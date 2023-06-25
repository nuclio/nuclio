/*
Copyright 2023 The Nuclio Authors.

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
	"github.com/nuclio/nuclio/pkg/processor/trigger"

	"github.com/nuclio/errors"
	"github.com/nuclio/logger"
	"github.com/prometheus/client_golang/prometheus"
)

type TriggerGatherer struct {
	trigger                                     trigger.Trigger
	logger                                      logger.Logger
	handledEventsTotal                          *prometheus.CounterVec
	workerAllocationCount                       prometheus.Counter
	workerAllocationTotal                       *prometheus.CounterVec
	workerAllocationWaitDurationMilliSecondsSum prometheus.Counter
	workerAllocationWorkersAvailablePercentage  prometheus.Counter
	prevStatistics                              trigger.Statistics
}

func NewTriggerGatherer(instanceName string,
	trigger trigger.Trigger,
	logger logger.Logger,
	metricRegistry *prometheus.Registry) (*TriggerGatherer, error) {

	newTriggerGatherer := &TriggerGatherer{
		trigger: trigger,
		logger:  logger.GetChild("gatherer"),
	}

	// base labels for handle events
	labels := prometheus.Labels{
		"instance":     instanceName,
		"trigger_kind": trigger.GetKind(),
		"trigger_id":   trigger.GetID(),
		"namespace":    trigger.GetNamespace(),
		"function":     trigger.GetFunctionName(),
		"project":      trigger.GetProjectName(),
	}

	newTriggerGatherer.handledEventsTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name:        "nuclio_processor_handled_events_total",
		Help:        "Total number of handled events",
		ConstLabels: labels,
	}, []string{"result"})

	newTriggerGatherer.workerAllocationTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name:        "nuclio_processor_worker_allocation_total",
		Help:        "Total number of worker allocations, by result",
		ConstLabels: labels,
	}, []string{"result"})

	newTriggerGatherer.workerAllocationCount = prometheus.NewCounter(prometheus.CounterOpts{
		Name:        "nuclio_processor_worker_allocation_count",
		Help:        "Total number of worker_allocations",
		ConstLabels: labels,
	})

	newTriggerGatherer.workerAllocationWaitDurationMilliSecondsSum = prometheus.NewCounter(prometheus.CounterOpts{
		Name:        "nuclio_processor_worker_allocation_wait_duration_milliseconds_sum",
		Help:        "Total number of milliseconds spent waiting for a worker",
		ConstLabels: labels,
	})

	newTriggerGatherer.workerAllocationWorkersAvailablePercentage = prometheus.NewCounter(prometheus.CounterOpts{
		Name:        "nuclio_processor_worker_allocation_workers_available_percentage",
		Help:        "Percent of workers available when an allocation occurred",
		ConstLabels: labels,
	})

	for _, collector := range []prometheus.Collector{
		newTriggerGatherer.handledEventsTotal,
		newTriggerGatherer.workerAllocationTotal,
		newTriggerGatherer.workerAllocationCount,
		newTriggerGatherer.workerAllocationWaitDurationMilliSecondsSum,
		newTriggerGatherer.workerAllocationWorkersAvailablePercentage,
	} {
		if err := metricRegistry.Register(collector); err != nil {
			return nil, errors.Wrap(err, "Failed to register collector")
		}
	}

	newTriggerGatherer.logger.DebugWith("Trigger gatherer created",
		"triggerID", trigger.GetID(),
		"triggerKind", trigger.GetKind())

	return newTriggerGatherer, nil
}

func (tg *TriggerGatherer) Gather() error {

	// read current stats
	currentStatistics := *tg.trigger.GetStatistics()

	// diff from previous to get this period, DiffFrom returns a full copy of statistics,
	// which can be accessed without atomicity concerns
	diffStatistics := currentStatistics.DiffFrom(&tg.prevStatistics)

	tg.handledEventsTotal.With(prometheus.Labels{
		"result": "success",
	}).Add(float64(diffStatistics.EventsHandledSuccessTotal))

	tg.handledEventsTotal.With(prometheus.Labels{
		"result": "failure",
	}).Add(float64(diffStatistics.EventsHandledFailureTotal))

	tg.workerAllocationCount.Add(
		float64(diffStatistics.WorkerAllocatorStatistics.WorkerAllocationCount))
	tg.workerAllocationWaitDurationMilliSecondsSum.Add(
		float64(diffStatistics.WorkerAllocatorStatistics.WorkerAllocationWaitDurationMilliSecondsSum))
	tg.workerAllocationWorkersAvailablePercentage.Add(
		float64(diffStatistics.WorkerAllocatorStatistics.WorkerAllocationWorkersAvailablePercentage))

	tg.workerAllocationTotal.With(prometheus.Labels{
		"result": "success_immediate",
	}).Add(float64(diffStatistics.WorkerAllocatorStatistics.WorkerAllocationSuccessImmediateTotal))

	tg.workerAllocationTotal.With(prometheus.Labels{
		"result": "success_after_wait",
	}).Add(float64(diffStatistics.WorkerAllocatorStatistics.WorkerAllocationSuccessAfterWaitTotal))

	tg.workerAllocationTotal.With(prometheus.Labels{
		"result": "error_timeout",
	}).Add(float64(diffStatistics.WorkerAllocatorStatistics.WorkerAllocationTimeoutTotal))

	tg.prevStatistics = currentStatistics

	return nil
}
