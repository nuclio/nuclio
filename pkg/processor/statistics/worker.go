package statistics

import (
	"strconv"

	"github.com/nuclio/nuclio/pkg/errors"

	"github.com/nuclio/nuclio/pkg/processor/eventsource"
	"github.com/nuclio/nuclio/pkg/processor/runtime"
	"github.com/nuclio/nuclio/pkg/processor/worker"
	"github.com/prometheus/client_golang/prometheus"
)

type workerGatherer struct {
	worker                                 *worker.Worker
	prevRuntimeStatistics                  runtime.Statistics
	handledEventsDurationMillisecondsSum   prometheus.Counter
	handledEventsDurationMillisecondsCount prometheus.Counter
}

func newWorkerGatherer(instanceName string,
	eventSource eventsource.EventSource,
	worker *worker.Worker,
	metricRegistry *prometheus.Registry) (*workerGatherer, error) {

	newWorkerGatherer := &workerGatherer{
		worker: worker,
	}

	// base labels for handle events
	labels := prometheus.Labels{
		"instance":           instanceName,
		"event_source_class": eventSource.GetClass(),
		"event_source_kind":  eventSource.GetKind(),
		"event_source_id":    eventSource.GetID(),
		"worker_index":       strconv.Itoa(worker.GetIndex()),
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

func (wg *workerGatherer) Gather() error {

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
