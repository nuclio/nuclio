package scaler

import (
	"github.com/nuclio/nuclio/pkg/errors"
	"sync"
	"time"

	"github.com/nuclio/nuclio/pkg/functionconfig"
	nuclioio_client "github.com/nuclio/nuclio/pkg/platform/kube/client/clientset/versioned"

	"github.com/nuclio/logger"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type functionMap map[functionMetricKey]*functionconfig.Spec

type Autoscale struct {
	logger            logger.Logger
	namespace         string
	metricsChannel    <-chan metricEntry
	functionMetricMap map[functionMetricKey][]metricEntry
	metricsMutex      sync.Mutex
	nuclioClientSet   nuclioio_client.Interface
}

func NewAutoScaler(parentLogger logger.Logger,
	namespace string,
	nuclioClientSet nuclioio_client.Interface,
	ch <-chan metricEntry) (*Autoscale, error) {
	return &Autoscale{
		logger:            parentLogger.GetChild("autoscale"),
		namespace:         namespace,
		metricsChannel:    ch,
		functionMetricMap: make(map[functionMetricKey][]metricEntry),
		nuclioClientSet:   nuclioClientSet,
	}, nil
}

func (as *Autoscale) CheckFunctionsToScale(t time.Time, runningFunctions functionMap) {
	as.metricsMutex.Lock()
	defer as.metricsMutex.Unlock()

	as.logger.Debug("Checking to scale")
	for key, metrics := range as.functionMetricMap {

		// TODO better than 4 continue
		if _, found := runningFunctions[key]; !found {
			continue
		}

		if runningFunctions[key].Metrics == nil {
			as.logger.Debug("No metric resources defined for the function", "functionName", key.functionName)
			continue
		}

		for _, metric := range runningFunctions[key].Metrics {

			if metric.SourceType != key.sourceType {
				continue
			}

			window, err := time.ParseDuration(metric.WindowSize)
			if err != nil {
				as.logger.DebugWith("Failed to parse window size for function", "functionName", key.functionName)
				continue
			}

			// this will give out the greatest delta
			var minMetric *metricEntry
			for idx, stat := range metrics {

				if stat.value < metric.ThresholdValue && minMetric == nil {
					minMetric = &metrics[idx]
				} else if stat.value >= metric.ThresholdValue {
					minMetric = nil
				}
			}

			if minMetric != nil && t.Sub(minMetric.timestamp) > window {
				as.logger.DebugWith("Metric value is below threshold and passed the window",
					"metricValue", minMetric.value,
					"function", key.functionName,
					"threshold", metric.ThresholdValue,
					"deltaSeconds", t.Sub(minMetric.timestamp).Seconds(),
					"windowSize", metric.WindowSize)
				as.ScaleToZero(key.namespace, key.functionName)
				as.removeMetricEntry(key)
			} else {
				if minMetric != nil {
					as.logger.DebugWith("Function values are still in window",
						"functionName", key.functionName,
						"value", minMetric.value,
						"threshold", metric.ThresholdValue,
						"deltaSeconds", t.Sub(minMetric.timestamp).Seconds(),
						"windowSize", metric.WindowSize)
				} else {
					as.logger.Debug("Function metrics are above threshold")
				}
				//TODO clean all metrics with time earlier than now minus window size
			}
		}
	}
}

func (as *Autoscale) ScaleToZero(namespace string, functionName string) {
	as.logger.DebugWith("Scaling to zero", "functionName", functionName)
	function, err := as.nuclioClientSet.NuclioV1beta1().Functions(as.namespace).Get(functionName, metav1.GetOptions{})
	if err != nil {
		as.logger.WarnWith("Failed to get nuclio function", "functionName", functionName, "err", err)
		return
	}

	// this has the nice property of disabling hpa as well
	function.Spec.MaxReplicas = 0
	function.Spec.MinReplicas = 0
	updatedFunction, err := as.nuclioClientSet.NuclioV1beta1().Functions(namespace).Update(function)
	if err != nil {
		as.logger.WarnWith("Failed to update function", "functionName", functionName, "err", err)
		return
	}

	// TODO retry
	// sanity check
	if updatedFunction.Spec.MinReplicas != 0 || updatedFunction.Spec.MaxReplicas != 0 {
		as.logger.WarnWith("Function was not properly scaled to zero",
			"functionName", functionName,
			"err", err)
		return
	}
}

func (as *Autoscale) AddMetricEntry(key functionMetricKey, entry metricEntry) {
	as.metricsMutex.Lock()
	defer as.metricsMutex.Unlock()
	as.functionMetricMap[key] = append(as.functionMetricMap[key], entry)
}

func (as *Autoscale) removeMetricEntry(key functionMetricKey) {
	as.metricsMutex.Lock()
	defer as.metricsMutex.Unlock()
	delete(as.functionMetricMap, key)
}

func (as *Autoscale) buildFunctionsMap() (functionMap, error) {
	functions, err := as.nuclioClientSet.NuclioV1beta1().Functions(as.namespace).List(metav1.ListOptions{})
	if err != nil {
		return nil, errors.Wrap(err, "Failed to list functions")
	}

	resultFunctionMap := make(functionMap)

	// build a map of functions and metric types
	for _, function := range functions.Items {
		for _, metricSource := range function.Spec.Metrics {
			if metricSource.SourceType == "" {
				as.logger.WarnWith("Metric is specified but no source", "functionName", function.Name)

				// no need to keep this item around
				continue
			}
			key := functionMetricKey{
				functionName: function.Name,
				namespace: function.Namespace,
				sourceType: metricSource.SourceType,
			}
			resultFunctionMap[key] = &function.Spec
		}
	}
	return resultFunctionMap, nil
}

func (as *Autoscale) start() {
	go func() {
		for metric := range as.metricsChannel {
			as.AddMetricEntry(metric.functionMetricKey, metric)
		}
	}()

	go func() {
		functionsMap, err := as.buildFunctionsMap()
		if err != nil {
			as.logger.WarnWith("Failed to build function map")
		}
		as.CheckFunctionsToScale(time.Now(), functionsMap)
	}()
}
