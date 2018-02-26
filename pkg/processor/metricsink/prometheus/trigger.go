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
	"github.com/nuclio/nuclio/pkg/errors"
	"github.com/nuclio/nuclio/pkg/processor/trigger"

	"github.com/prometheus/client_golang/prometheus"
)

type TriggerGatherer struct {
	trigger            trigger.Trigger
	handledEventsTotal *prometheus.CounterVec
	prevStatistics     trigger.Statistics
}

func NewTriggerGatherer(instanceName string,
	trigger trigger.Trigger,
	metricRegistry *prometheus.Registry) (*TriggerGatherer, error) {

	newTriggerGatherer := &TriggerGatherer{
		trigger: trigger,
	}

	// base labels for handle events
	labels := prometheus.Labels{
		"instance":      instanceName,
		"trigger_class": trigger.GetClass(),
		"trigger_kind":  trigger.GetKind(),
		"trigger_id":    trigger.GetID(),
	}

	newTriggerGatherer.handledEventsTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name:        "nuclio_processor_handled_events_total",
		Help:        "Total number of handled events",
		ConstLabels: labels,
	}, []string{"result"})

	if err := metricRegistry.Register(newTriggerGatherer.handledEventsTotal); err != nil {
		return nil, errors.Wrap(err, "Failed to register handled events metric")
	}

	return newTriggerGatherer, nil
}

func (esg *TriggerGatherer) Gather() error {

	// read current stats
	currentStatistics := *esg.trigger.GetStatistics()

	// diff from previous to get this period
	diffStatistics := currentStatistics.DiffFrom(&esg.prevStatistics)

	esg.handledEventsTotal.With(prometheus.Labels{
		"result": "success",
	}).Add(float64(diffStatistics.EventsHandleSuccessTotal))

	esg.handledEventsTotal.With(prometheus.Labels{
		"result": "failure",
	}).Add(float64(diffStatistics.EventsHandleFailureTotal))

	esg.prevStatistics = currentStatistics

	return nil
}
