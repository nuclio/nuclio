package statistics

import (
	"github.com/nuclio/nuclio/pkg/errors"

	"github.com/nuclio/nuclio/pkg/processor/eventsource"
	"github.com/prometheus/client_golang/prometheus"
)

type eventSourceGatherer struct {
	eventSource   eventsource.EventSource
	handledEvents *prometheus.GaugeVec
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

	newEventSourceGatherer.handledEvents = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name:        "nuclio_processor_handled_events",
		Help:        "Number of handled events",
		ConstLabels: labels,
	}, []string{"result"})

	if err := metricRegistry.Register(newEventSourceGatherer.handledEvents); err != nil {
		return nil, errors.Wrap(err, "Failed to register handled events metric")
	}

	return newEventSourceGatherer, nil
}

func (esg *eventSourceGatherer) Gather() error {

	// read stats (copy)
	statistics := *esg.eventSource.GetStatistics()

	esg.handledEvents.With(prometheus.Labels{
		"result": "success",
	}).Set(float64(statistics.EventsHandleSuccess))

	esg.handledEvents.With(prometheus.Labels{
		"result": "failure",
	}).Set(float64(statistics.EventsHandleFailure))

	return nil
}
