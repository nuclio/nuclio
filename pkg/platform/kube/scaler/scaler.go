package scaler

import (
	"github.com/nuclio/logger"
	nuclioio_client "github.com/nuclio/nuclio/pkg/platform/kube/client/clientset/versioned"
	"github.com/nuclio/nuclio/pkg/version"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	metricsv1 "k8s.io/metrics/pkg/client/clientset_generated/clientset"
	custommetricsv1 "k8s.io/metrics/pkg/client/custom_metrics"
	"time"
)

type functionMetricKey struct {
	sourceType string
	namespace string
	functionName string
}

type metricEntry struct {
	timestamp time.Time
	value     int64
	functionMetricKey functionMetricKey
}

type Scaler struct {
	logger                 logger.Logger
	namespace              string
	restConfig             *rest.Config
	kubeClientSet          kubernetes.Interface
	metricsPoller          *metricsOperator
	metricsClientset       *metricsv1.Clientset
	customMetricsClientSet custommetricsv1.CustomMetricsClient
	nuclioClientSet        nuclioio_client.Interface
	autoscaler             *Autoscale
	scaleInterval          time.Duration
}

func NewScaler(parentLogger logger.Logger,
	namespace string,
	kubeClientSet kubernetes.Interface,
	metricsClientSet *metricsv1.Clientset,
	nuclioClientSet *nuclioio_client.Clientset,
	customMetricsClientSet custommetricsv1.CustomMetricsClient,
    scaleInterval time.Duration,
    metricsInterval time.Duration,
	resyncInterval time.Duration) (*Scaler, error) {

	// replace "*" with "", which is actually "all" in kube-speak
	if namespace == "*" {
		namespace = ""
	}

	scaler := &Scaler{
		logger:            parentLogger,
		namespace:         namespace,
		kubeClientSet:     kubeClientSet,
		metricsClientset:  metricsClientSet,
		customMetricsClientSet: customMetricsClientSet,
		nuclioClientSet:   nuclioClientSet,
		scaleInterval:     scaleInterval,
	}

	var err error

	// a shared metricsChannel between autoscaler and sources
	metricsChannel := make(chan metricEntry)
	scaler.metricsPoller, err = NewMetricsPoller(scaler.logger, scaler, metricsChannel, metricsInterval, namespace)
	if err != nil {
		return nil, err
	}
	scaler.autoscaler, err = NewAutoScaler(scaler.logger, namespace, nuclioClientSet, metricsChannel)
	if err != nil {
		return nil, err
	}

	// log version info
	version.Log(scaler.logger)
	return scaler, nil
}

func (s *Scaler) Start() error {
	s.logger.InfoWith("Starting", "namespace", s.namespace)

	err := s.metricsPoller.start()
	if err != nil {
		return err
	}

	// start a listener and handler of all metrics
	s.autoscaler.start()

	return nil
}
