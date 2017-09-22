package statistics

import (
	"github.com/nuclio/nuclio/pkg/errors"

	"github.com/nuclio/nuclio/pkg/processor/eventsource"
	"github.com/prometheus/client_golang/prometheus"
)

type eventSourceGatherer struct {
	eventSource        eventsource.EventSource
	handledEventsTotal *prometheus.CounterVec
	prevStatistics     eventsource.Statistics
}

func newEventSourceGatherer(instanceName string,
	eventSource eventsource.EventSource,
	metricRegistry *prometheus.Registry) (*eventSourceGatherer, error) {

	newEventSourceGatherer := &eventSourceGatherer{
		eventSource: eventSource,
	}

	// base labels for handle events
	labels := prometheus.Labels{
		"instance":           instanceName,
		"event_source_class": eventSource.GetClass(),
		"event_source_kind":  eventSource.GetKind(),
		"event_source_id":    eventSource.GetID(),
	}

	newEventSourceGatherer.handledEventsTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name:        "nuclio_processor_handled_events_total",
		Help:        "Total number of handled events",
		ConstLabels: labels,
	}, []string{"result"})

	if err := metricRegistry.Register(newEventSourceGatherer.handledEventsTotal); err != nil {
		return nil, errors.Wrap(err, "Failed to register handled events metric")
	}

	return newEventSourceGatherer, nil
}

func (esg *eventSourceGatherer) Gather() error {

	// read current stats
	currentStatistics := *esg.eventSource.GetStatistics()

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
