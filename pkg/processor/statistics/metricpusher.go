package statistics

import (
	"strconv"
	"time"

	"github.com/nuclio/nuclio/pkg/processor/eventsource"

	"github.com/nuclio/nuclio-sdk"
	"github.com/nuclio/nuclio/pkg/errors"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/push"
	"github.com/spf13/viper"
	"os"
)

type eventSourceProvider interface {
	GetEventSources() []eventsource.EventSource
}

type MetricPusher struct {
	logger             nuclio.Logger
	eventSources       []eventsource.EventSource
	handledEventsGauge *prometheus.GaugeVec
	metricRegistry     *prometheus.Registry
	jobName            string
	instanceName       string
	pushGatewayURL     string
	pushInterval       int
}

func NewMetricPusher(parentLogger nuclio.Logger,
	eventSourceProvider eventSourceProvider,
	configuration *viper.Viper) (*MetricPusher, error) {

	newMetricPusher := &MetricPusher{
		logger:         parentLogger.GetChild("metrics").(nuclio.Logger),
		eventSources:   eventSourceProvider.GetEventSources(),
		metricRegistry: prometheus.NewRegistry(),
	}

	// read configuration
	if err := newMetricPusher.readConfiguration(configuration); err != nil {
		return nil, errors.Wrap(err, "Failed to read configuration")
	}

	// create a bunch of prometheus metrics which we will populate periodically
	if err := newMetricPusher.registerMetrics(); err != nil {
		return nil, errors.Wrap(err, "Failed to register metrics")
	}

	newMetricPusher.logger.InfoWith("Metrics pusher created",
		"jobName", newMetricPusher.jobName,
		"instanceName", newMetricPusher.instanceName,
		"pushGatewayURL", newMetricPusher.pushGatewayURL,
		"pushInterval", newMetricPusher.pushInterval)

	return newMetricPusher, nil
}

func (mp *MetricPusher) Start() error {
	go mp.periodicallyPushMetrics()

	return nil
}

func (mp *MetricPusher) readConfiguration(configuration *viper.Viper) error {
	pushGatewayURLDefault := os.Getenv("NUCLIO_PROM_PUSH_GATEWAY_URL")
	if pushGatewayURLDefault == "" {
		pushGatewayURLDefault = "http://nuclio-prometheus-pushgateway:9091"
	}

	pushInterval := os.Getenv("NUCLIO_PROM_PUSH_INTERVAL")
	if pushInterval == "" {
		pushInterval = "10"
	}

	configuration.SetDefault("job_name", os.Getenv("NUCLIO_FUNCTION_NAME"))
	configuration.SetDefault("instance", os.Getenv("NUCLIO_FUNCTION_INSTANCE"))
	configuration.SetDefault("push_gateway_url", pushGatewayURLDefault)
	configuration.SetDefault("push_interval", pushInterval)

	mp.pushGatewayURL = configuration.GetString("push_gateway_url")
	mp.jobName = configuration.GetString("job_name")
	mp.instanceName = configuration.GetString("instance")
	mp.pushInterval = configuration.GetInt("push_interval")

	return nil
}

func (mp *MetricPusher) registerMetrics() error {
	mp.handledEventsGauge = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "nuclio_processor_handled_events",
		Help: "Number of handled events",
	}, []string{"instance", "event_source_class", "event_source_kind", "worker_index", "result"})

	// register all the metrics
	for _, collector := range []prometheus.Collector{mp.handledEventsGauge} {
		if err := mp.metricRegistry.Register(collector); err != nil {
			return errors.Wrap(err, "Failed to register metric")
		}
	}

	return nil
}

func (mp *MetricPusher) periodicallyPushMetrics() {

	for {

		// every 10 seconds
		time.Sleep(10 * time.Second)

		// gather the metrics from the event sources - this will update the metrics
		// from counters internally held by event sources and their child objects
		mp.gatherEventSourceMetrics()

		// AddFromGatherer is used here rather than FromGatherer to not delete a
		// previously pushed success timestamp in case of a failure of this
		// backup.
		if err := push.AddFromGatherer(mp.jobName, nil, mp.pushGatewayURL, mp.metricRegistry); err != nil {
			mp.logger.WarnWith("Failed to push metrics", "err", err)
		}
	}
}

func (mp *MetricPusher) gatherEventSourceMetrics() {
	for _, eventSource := range mp.eventSources {
		for _, worker := range eventSource.GetWorkers() {

			// copy the current state of worker statistics
			workerStatistics := worker.GetStatistics()

			//
			// TODO: proper label management
			//

			// generate worker labels
			workerLabels0 := prometheus.Labels{
				"instance":           mp.instanceName,
				"event_source_class": eventSource.GetClass(),
				"event_source_kind":  eventSource.GetKind(),
				"worker_index":       strconv.Itoa(worker.GetIndex()),
				"result":             "success",
			}

			// increment the appropriate counters
			mp.handledEventsGauge.With(workerLabels0).Set(float64(workerStatistics.EventsHandleSuccess))

			// generate worker labels
			workerLabels1 := prometheus.Labels{
				"instance":           mp.instanceName,
				"event_source_class": eventSource.GetClass(),
				"event_source_kind":  eventSource.GetKind(),
				"worker_index":       strconv.Itoa(worker.GetIndex()),
				"result":             "failure",
			}

			// increment the appropriate counters
			mp.handledEventsGauge.With(workerLabels1).Set(float64(workerStatistics.EventsHandleError))
		}
	}
}
