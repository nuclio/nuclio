package scaler

import (
	"github.com/nuclio/logger"
	"github.com/nuclio/nuclio/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"time"
)

type metricsOperator struct {
	logger       logger.Logger
	scaler       *Scaler
	statsChannel chan metricEntry
	ticker       *time.Ticker
	namespace    string
}

func NewMetricsPoller(parentLogger logger.Logger,
	scaler *Scaler,
	statsChannel chan metricEntry,
	interval time.Duration,
	namespace string) (*metricsOperator, error) {
	var err error

	loggerInstance := parentLogger.GetChild("metrics")

	ticker := time.NewTicker(interval)

	newMetricsOperator := &metricsOperator{
		logger:       loggerInstance,
		scaler:       scaler,
		statsChannel: statsChannel,
		ticker:       ticker,
		namespace:    namespace,
	}
	if err != nil {
		return nil, errors.Wrap(err, "Failed to create project operator")
	}

	return newMetricsOperator, nil
}

func (po *metricsOperator) getFunctionMetrics() {
	kind := "nuclio_processor_handled_events"
	schemaGroupKind := schema.GroupKind{Group: "", Kind: "Function"}
	functionLabels := labels.Nothing()
	c := po.scaler.customMetricsClientSet.NamespacedMetrics(po.namespace)
	cm, err := c.
		GetForObjects(schemaGroupKind,
		functionLabels,
		kind)
	if err != nil {
		po.logger.ErrorWith("Error in getting custom metrics", "err", err)
		return
	}

	for _, item := range cm.Items {

		po.logger.DebugWith("Publishing new metric",
			"function", item.DescribedObject.Name,
			"value", item.Value.MilliValue())
		newEntry := metricEntry{
			timestamp:    time.Now(),
			value:        item.Value.MilliValue(),
			functionMetricKey: functionMetricKey{
				namespace: item.DescribedObject.Namespace,
				functionName: item.DescribedObject.Name,
				sourceType: "nuclio_processor_handled_events",
			},
		}
		po.statsChannel <- newEntry
	}
}

func (po *metricsOperator) getCPUStats() {
	podMetrics, err := po.scaler.metricsClientset.MetricsV1beta1().PodMetricses(po.namespace).List(metav1.ListOptions{})
	if err != nil {
		po.logger.ErrorWith("Failed to list pod metrics", "err", err)
		return
	}
	po.logger.DebugWith("Got metrics", "len", len(podMetrics.Items))

	pods, err := po.scaler.kubeClientSet.CoreV1().Pods(po.namespace).List(metav1.ListOptions{
		LabelSelector: "nuclio.io/class=function",
	})
	if err != nil {
		po.logger.ErrorWith("Failed to list pods", "err", err)
		return
	}
	po.logger.DebugWith("Found function pods", "len", len(pods.Items))

	for _, podMetric := range podMetrics.Items {

		functionName, err := po.getFunctionNameByPodName(pods, podMetric.Name)
		if err != nil {
			continue
		}
		po.logger.DebugWith("Got function name", "name", functionName)
		for _, container := range podMetric.Containers {
			int64Val := container.Usage.Cpu().MilliValue()

			po.logger.DebugWith("Container status", "cpu", container.Usage.Cpu())
			newEntry := metricEntry{
				timestamp:    time.Now(),
				value:        int64Val,
				functionMetricKey: functionMetricKey{
					namespace: po.namespace,
					functionName: functionName,
					sourceType: "cpu",
				},
			}

			// TODO change to AddEntry
			po.statsChannel <- newEntry
		}
	}
}

func (po *metricsOperator) getFunctionNameByPodName(podList *corev1.PodList, name string) (string, error) {
	pod, err := po.getPodByName(podList, name)
	if err != nil {
		return "", err
	}
	functionName := pod.Labels["nuclio.io/function-name"]
	if functionName == "" {
		return "", errors.New("Failed to get function name")
	}
	return functionName, nil
}

func (po *metricsOperator) getPodByName(podList *corev1.PodList, name string) (*corev1.Pod, error) {
	for _, pod := range podList.Items {
		if pod.Name == name {
			return &pod, nil
		}
	}
	return nil, errors.New("Failed to locate pod in list")
}

func (po *metricsOperator) start() error {
	go func() {
		for range po.ticker.C {
			po.getFunctionMetrics()
		}
	}()

	return nil
}
