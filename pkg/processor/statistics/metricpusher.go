package statistics

import (
	"os"
	"time"

	"github.com/nuclio/nuclio/pkg/errors"
	"github.com/nuclio/nuclio/pkg/processor/eventsource"

	"github.com/nuclio/nuclio-sdk"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/push"
	"github.com/spf13/viper"
)

type eventSourceProvider interface {
	GetEventSources() []eventsource.EventSource
}

type MetricPusher struct {
	logger              nuclio.Logger
	metricRegistry      *prometheus.Registry
	jobName             string
	instanceName        string
	pushGatewayURL      string
	pushIntervalSeconds int
	gatherers           []Gatherer
}

func NewMetricPusher(parentLogger nuclio.Logger,
	eventSourceProvider eventSourceProvider,
	configuration *viper.Viper) (*MetricPusher, error) {

	newMetricPusher := &MetricPusher{
		logger:         parentLogger.GetChild("metrics").(nuclio.Logger),
		metricRegistry: prometheus.NewRegistry(),
	}

	// read configuration
	if err := newMetricPusher.readConfiguration(configuration); err != nil {
		return nil, errors.Wrap(err, "Failed to read configuration")
	}

	// create a bunch of prometheus metrics which we will populate periodically
	if err := newMetricPusher.createGatherers(eventSourceProvider); err != nil {
		return nil, errors.Wrap(err, "Failed to register metrics")
	}

	newMetricPusher.logger.InfoWith("Metrics pusher created",
		"jobName", newMetricPusher.jobName,
		"instanceName", newMetricPusher.instanceName,
		"pushGatewayURL", newMetricPusher.pushGatewayURL,
		"pushInterval", newMetricPusher.pushIntervalSeconds)

	return newMetricPusher, nil
}

func (mp *MetricPusher) Start() error {
	go mp.periodicallyPushMetrics()

	return nil
}

func (mp *MetricPusher) readConfiguration(configuration *viper.Viper) error {
	pushGatewayURLDefault := os.Getenv("NUCLIO_PROM_PUSH_GATEWAY_URL")
	if pushGatewayURLDefault == "" {
		pushGatewayURLDefault = "http://prometheus-prometheus-pushgateway:9091"
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
	mp.pushIntervalSeconds = configuration.GetInt("push_interval")

	return nil
}

func (mp *MetricPusher) createGatherers(eventSourceProvider eventSourceProvider) error {

	for _, eventSource := range eventSourceProvider.GetEventSources() {

		// create a gatherer for the event source
		eventSourceGatherer, err := newEventSourceGatherer(mp.instanceName, eventSource, mp.metricRegistry)
		if err != nil {
			return errors.Wrap(err, "Failed to create event source gatherer")
		}

		mp.gatherers = append(mp.gatherers, eventSourceGatherer)

		// now add workers
		for _, worker := range eventSource.GetWorkers() {
			workerGatherer, err := newWorkerGatherer(mp.instanceName, eventSource, worker, mp.metricRegistry)
			if err != nil {
				return errors.Wrap(err, "Failed to create worker gatherer")
			}

			mp.gatherers = append(mp.gatherers, workerGatherer)
		}
	}

	return nil
}

func (mp *MetricPusher) periodicallyPushMetrics() {

	for {

		// every mp.pushInterval seconds
		time.Sleep(time.Duration(mp.pushIntervalSeconds) * time.Second)

		// gather the metrics from the event sources - this will update the metrics
		// from counters internally held by event sources and their child objects
		mp.gather()

		// AddFromGatherer is used here rather than FromGatherer to not delete a
		// previously pushed success timestamp in case of a failure of this
		// backup.
		if err := push.AddFromGatherer(mp.jobName, nil, mp.pushGatewayURL, mp.metricRegistry); err != nil {
			mp.logger.WarnWith("Failed to push metrics", "err", err)
		}
	}
}

func (mp *MetricPusher) gather() error {

	for _, gatherer := range mp.gatherers {
		if err := gatherer.Gather(); err != nil {
			return err
		}
	}

	return nil
}
