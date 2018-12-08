package scaler

import (
	"time"

	"github.com/nuclio/nuclio/pkg/errors"
	"github.com/nuclio/nuclio/pkg/functionconfig"
	nuclioio_client "github.com/nuclio/nuclio/pkg/platform/kube/client/clientset/versioned"
	"github.com/nuclio/nuclio/pkg/platformconfig"
	"github.com/nuclio/nuclio/pkg/version"

	"github.com/nuclio/logger"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	metricsv1 "k8s.io/metrics/pkg/client/clientset_generated/clientset"
	custommetricsv1 "k8s.io/metrics/pkg/client/custom_metrics"
)

type Scaler struct {
	logger                 logger.Logger
	namespace              string
	restConfig             *rest.Config
	kubeClientSet          kubernetes.Interface
	metricsPoller          *metricsPoller
	metricsClientset       *metricsv1.Clientset
	customMetricsClientSet custommetricsv1.CustomMetricsClient
	nuclioClientSet        *nuclioio_client.Clientset
	autoscaler             *autoscaler
}

type metricEntry struct {
	timestamp    time.Time
	value        int64
	functionName string
	metricName   string
}

func NewScaler(parentLogger logger.Logger,
	namespace string,
	kubeClientSet kubernetes.Interface,
	metricsClientSet *metricsv1.Clientset,
	nuclioClientSet *nuclioio_client.Clientset,
	customMetricsClientSet custommetricsv1.CustomMetricsClient,
	platformConfig *platformconfig.Configuration) (*Scaler, error) {

	// replace "*" with "", which is actually "all" in kube-speak
	if namespace == "*" {
		namespace = ""
	}

	scaler := &Scaler{
		logger:                 parentLogger,
		namespace:              namespace,
		kubeClientSet:          kubeClientSet,
		metricsClientset:       metricsClientSet,
		customMetricsClientSet: customMetricsClientSet,
		nuclioClientSet:        nuclioClientSet,
	}

	var err error

	metricName := platformConfig.ScaleToZero.MetricName
	scaleWindow, err := time.ParseDuration(platformConfig.ScaleToZero.WindowSize)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to parse scale window size")
	}

	metricInterval, err := time.ParseDuration(platformConfig.ScaleToZero.PollerInterval)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to parse metric poller interval")
	}

	scaleInterval, err := time.ParseDuration(platformConfig.ScaleToZero.ScalerInterval)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to parse scaler interval")
	}

	err = scaler.createAutoScaler(metricName, scaleWindow, scaleInterval)
	if err != nil {
		return nil, err
	}

	scaler.metricsPoller, err = newMetricsPoller(scaler.logger,
		scaler,
		scaler.autoscaler,
		metricInterval,
		metricName,
		namespace)
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
		return errors.Wrap(err, "Failed to start metrics poller")
	}

	// start a listener and handler of all metrics
	err = s.autoscaler.start()
	if err != nil {
		return errors.Wrap(err, "Failed to start auto scaler")
	}

	return nil
}

func (s *Scaler) scaleFunctionToZero(namespace string, functionName string) {
	s.logger.DebugWith("Scaling to zero", "functionName", functionName)
	function, err := s.nuclioClientSet.NuclioV1beta1().Functions(s.namespace).Get(functionName, metav1.GetOptions{})
	if err != nil {
		s.logger.WarnWith("Failed to get nuclio function", "functionName", functionName, "err", err)
	}

	// this has the nice property of disabling hpa as well
	function.Status.State = functionconfig.FunctionStateScaledToZero
	_, err = s.nuclioClientSet.NuclioV1beta1().Functions(namespace).Update(function)
	if err != nil {
		s.logger.WarnWith("Failed to update function", "functionName", functionName, "err", err)
	}
}

func (s *Scaler) createAutoScaler(metricName string,
	scaleWindow time.Duration,
	scaleInterval time.Duration) error {

	autoscaler, err := newAutoScaler(s.logger,
		s.namespace,
		s.nuclioClientSet,
		s,
		scaleInterval,
		scaleWindow,
		metricName)
	if err != nil {
		return errors.Wrap(err, "Failed to create auto scaler")
	}
	s.autoscaler = autoscaler
	return nil
}
