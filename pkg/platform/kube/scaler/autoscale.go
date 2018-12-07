package scaler

import (
	"sync"
	"time"

	"github.com/nuclio/nuclio/pkg/errors"
	"github.com/nuclio/nuclio/pkg/functionconfig"
	nuclioio_client "github.com/nuclio/nuclio/pkg/platform/kube/client/clientset/versioned"

	"github.com/nuclio/logger"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type functionMap map[string]*functionconfig.Spec
type functionMetricTypeMap map[string]map[string][]metricEntry
type scalerFunction func(namespace string, functionName string)

type Autoscale struct {
	logger          logger.Logger
	namespace       string
	metricsChannel  <-chan metricEntry
	metricsMap      functionMetricTypeMap
	metricsMutex    sync.Mutex
	nuclioClientSet nuclioio_client.Interface
	scalerFunction  scalerFunction
	metricType      string
	scaleInterval   time.Duration
	windowSize      time.Duration
}

func NewAutoScaler(parentLogger logger.Logger,
	namespace string,
	nuclioClientSet nuclioio_client.Interface,
	scalerFunction scalerFunction,
	scaleInterval time.Duration,
	windowSize time.Duration,
	metricType string,
	ch <-chan metricEntry) (*Autoscale, error) {
	childLogger := parentLogger.GetChild("autoscale")
	childLogger.DebugWith("Creating autoscaler",
		"namespace", namespace,
		"metricName", metricType)

	return &Autoscale{
		logger:          childLogger,
		namespace:       namespace,
		metricsChannel:  ch,
		metricsMap:      make(functionMetricTypeMap),
		nuclioClientSet: nuclioClientSet,
		scalerFunction:  scalerFunction,
		metricType:      metricType,
		windowSize:      windowSize,
		scaleInterval:   scaleInterval,
	}, nil
}

func (as *Autoscale) CheckFunctionsToScale(t time.Time, activeFunctions functionMap) {
	as.metricsMutex.Lock()
	defer as.metricsMutex.Unlock()

	for functionName := range activeFunctions {

		// currently only one type of metric supported from a platform configuration
		functionMetrics := as.metricsMap[functionName][as.metricType]

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
			as.scalerFunction(as.namespace, functionName)
			delete(as.metricsMap, functionName)
		} else {
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
			as.logger.DebugWith("Current metrics", "metrics", as.metricsMap[functionName])

			// TODO fix
			// as.metricsMap[functionName][as.metricType] = newMetrics
		}
	}
}

func (as *Autoscale) AddMetricEntry(functionName string, metricType string, entry metricEntry) {
	as.metricsMutex.Lock()
	defer as.metricsMutex.Unlock()
	if _, found := as.metricsMap[functionName]; !found {
		as.metricsMap[functionName] = make(map[string][]metricEntry)
	}
	as.metricsMap[functionName][metricType] = append(as.metricsMap[functionName][metricType], entry)
}

func (as *Autoscale) buildFunctionsMap() (functionMap, error) {
	functions, err := as.nuclioClientSet.NuclioV1beta1().Functions(as.namespace).List(metav1.ListOptions{})
	if err != nil {
		return nil, errors.Wrap(err, "Failed to list functions")
	}

	resultFunctionMap := make(functionMap)

	// build a map of functions and metric types
	for _, function := range functions.Items {
		if function.Status.State == functionconfig.FunctionStateScaleToZero {
			as.logger.WarnWith("Metric is specified but function currently scaled to zero",
				"functionName", function.Name)

			// no need to keep this item around
			continue
		}
		resultFunctionMap[function.Name] = &function.Spec
	}
	return resultFunctionMap, nil
}

func (as *Autoscale) start() {
	go func() {
		for metric := range as.metricsChannel {
			as.AddMetricEntry(metric.functionName, metric.metricName, metric)
		}
	}()

	ticker := time.NewTicker(as.scaleInterval)
	go func() {
		for range ticker.C {
			functionsMap, err := as.buildFunctionsMap()
			if err != nil {
				as.logger.WarnWith("Failed to build function map")
			}
			as.CheckFunctionsToScale(time.Now(), functionsMap)
		}
	}()
}
