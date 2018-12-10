package scaler

import (
	"time"

	"github.com/nuclio/nuclio/pkg/errors"
	"github.com/nuclio/nuclio/pkg/functionconfig"
	nuclioio_client "github.com/nuclio/nuclio/pkg/platform/kube/client/clientset/versioned"

	"github.com/nuclio/logger"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type functionMap map[string]*functionconfig.ConfigWithStatus
type functionMetricTypeMap map[string]map[string][]metricEntry

type autoscaler struct {
	logger          logger.Logger
	namespace       string
	metricsChannel  chan metricEntry
	metricsMap      functionMetricTypeMap
	nuclioClientSet nuclioio_client.Interface
	functionScaler  functionScaler
	metricName      string
	scaleInterval   time.Duration
	windowSize      time.Duration
}

func newAutoScaler(parentLogger logger.Logger,
	namespace string,
	nuclioClientSet nuclioio_client.Interface,
	functionScaler functionScaler,
	scaleInterval time.Duration,
	windowSize time.Duration,
	metricName string) (*autoscaler, error) {
	childLogger := parentLogger.GetChild("autoscale")
	childLogger.DebugWith("Creating autoscaler",
		"namespace", namespace,
		"metricName", metricName)

	return &autoscaler{
		logger:          childLogger,
		namespace:       namespace,
		metricsMap:      make(functionMetricTypeMap),
		nuclioClientSet: nuclioClientSet,
		functionScaler:  functionScaler,
		metricName:      metricName,
		windowSize:      windowSize,
		scaleInterval:   scaleInterval,
		metricsChannel:  make(chan metricEntry, 1024),
	}, nil
}

func (as *autoscaler) checkFunctionsToScale(t time.Time, activeFunctions functionMap) {
	for functionName, functionConfig := range activeFunctions {

		if functionConfig.Status.State == functionconfig.FunctionStateScaledToZero {

			// scaled to zero functions are not of interest, delete the data and move on
			delete(as.metricsMap, functionName)
			continue
		}

		// currently only one type of metric supported from a platform configuration
		functionMetrics := as.metricsMap[functionName][as.metricName]

		// this will give out the greatest delta
		var minMetric *metricEntry
		for idx, metric := range functionMetrics {

			if metric.value == 0 && minMetric == nil {
				minMetric = &functionMetrics[idx]
			} else if metric.value > 0 {
				minMetric = nil
			}
		}

		if minMetric != nil && t.Sub(minMetric.timestamp) > as.windowSize {
			as.logger.DebugWith("Metric value is below threshold and passed the window",
				"metricValue", minMetric.value,
				"function", functionName,
				"deltaSeconds", t.Sub(minMetric.timestamp).Seconds(),
				"windowSize", as.windowSize)

			as.functionScaler.scaleFunctionToZero(as.namespace, functionName)
			delete(as.metricsMap, functionName)
		} else if as.metricsMap[functionName][as.metricName] != nil {
			if minMetric != nil {
				as.logger.DebugWith("Function values are still in window",
					"functionName", functionName,
					"value", minMetric.value,
					"deltaSeconds", t.Sub(minMetric.timestamp).Seconds(),
					"windowSize", as.windowSize)
			} else {
				as.logger.Debug("Function metrics are above threshold")
			}

			// rebuild the slice, excluding any old metrics
			var newMetrics []metricEntry
			for _, metric := range functionMetrics {
				if t.Sub(metric.timestamp) <= as.windowSize {
					newMetrics = append(newMetrics, metric)
				}
			}
			as.metricsMap[functionName][as.metricName] = newMetrics
		}
	}
}

func (as *autoscaler) addMetricEntry(functionName string, metricType string, entry metricEntry) {
	if _, found := as.metricsMap[functionName]; !found {
		as.metricsMap[functionName] = make(map[string][]metricEntry)
	}
	as.metricsMap[functionName][metricType] = append(as.metricsMap[functionName][metricType], entry)
}

func (as *autoscaler) buildFunctionsMap() (functionMap, error) {
	functions, err := as.nuclioClientSet.NuclioV1beta1().Functions(as.namespace).List(metav1.ListOptions{})
	if err != nil {
		return nil, errors.Wrap(err, "Failed to list functions")
	}

	resultFunctionMap := make(functionMap)

	// build a map of functions with status
	for _, function := range functions.Items {
		resultFunctionMap[function.Name] = &functionconfig.ConfigWithStatus{
			Config: functionconfig.Config{
				Spec: function.Spec,
			},
			Status: function.Status,
		}
	}
	return resultFunctionMap, nil
}

func (as *autoscaler) reportMetric(metric metricEntry) error {

	// don't block, try and fail fast
	select {
	case as.metricsChannel <- metric:
		return nil
	default:
		as.logger.WarnWith("Failed to report metric",
			"functionName", metric.functionName,
			"metricName", metric.metricName)
	}
	return nil
}

func (as *autoscaler) start() error {
	ticker := time.NewTicker(as.scaleInterval)

	go func() {
		for {
			select {
			case <-ticker.C:
				functionsMap, err := as.buildFunctionsMap()
				if err != nil {
					as.logger.WarnWith("Failed to build function map")
				}
				as.checkFunctionsToScale(time.Now(), functionsMap)
			case metric := <-as.metricsChannel:
				as.addMetricEntry(metric.functionName, metric.metricName, metric)
			}
		}
	}()
	return nil
}
