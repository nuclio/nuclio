package scaler

import (
	"time"

	"github.com/nuclio/nuclio/pkg/errors"

	"github.com/nuclio/logger"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

type metricsPoller struct {
	logger               logger.Logger
	scaler               *Scaler
	metricReporter       metricReporter
	ticker               *time.Ticker
	namespace            string
	metricName           string
	functionPodNameCache map[string]string
}

func newMetricsPoller(parentLogger logger.Logger,
	scaler *Scaler,
	metricReporter metricReporter,
	interval time.Duration,
	metricName string,
	namespace string) (*metricsPoller, error) {
	var err error

	loggerInstance := parentLogger.GetChild("metrics")

	ticker := time.NewTicker(interval)

	newMetricsOperator := &metricsPoller{
		logger:               loggerInstance,
		scaler:               scaler,
		metricReporter:       metricReporter,
		ticker:               ticker,
		namespace:            namespace,
		metricName:           metricName,
		functionPodNameCache: make(map[string]string),
	}
	if err != nil {
		return nil, errors.Wrap(err, "Failed to create project operator")
	}

	return newMetricsOperator, nil
}

func (mp *metricsPoller) getFunctionMetrics() error {
	schemaGroupKind := schema.GroupKind{Group: "", Kind: "Function"}
	functionLabels := labels.Nothing()
	c := mp.scaler.customMetricsClientSet.NamespacedMetrics(mp.namespace)
	cm, err := c.
		GetForObjects(schemaGroupKind,
			functionLabels,
			mp.metricName)
	if err != nil {
		return errors.Wrap(err, "Failed to get custom metrics")
	}

	for _, item := range cm.Items {

		mp.logger.DebugWith("Publishing new metric",
			"function", item.DescribedObject.Name,
			"value", item.Value.MilliValue())
		newEntry := metricEntry{
			timestamp:    time.Now(),
			value:        item.Value.MilliValue(),
			functionName: item.DescribedObject.Name,
			metricName:   mp.metricName,
		}
		err := mp.metricReporter.reportMetric(newEntry)
		if err != nil {
			return errors.Wrap(err, "Failed to report metric")
		}
	}
	return nil
}

func (mp *metricsPoller) getCPUMetrics() error {
	podMetrics, err := mp.scaler.metricsClientset.MetricsV1beta1().PodMetricses(mp.namespace).List(metav1.ListOptions{})
	if err != nil {
		return errors.Wrap(err, "Failed to list pod metrics")
	}
	mp.logger.DebugWith("Got metrics", "len", len(podMetrics.Items))

	pods, err := mp.scaler.kubeClientSet.CoreV1().Pods(mp.namespace).List(metav1.ListOptions{
		LabelSelector: "nuclio.io/class=function",
	})
	if err != nil {
		return errors.Wrap(err, "Failed to list pods")
	}
	mp.logger.DebugWith("Found function pods", "len", len(pods.Items))

	for _, podMetric := range podMetrics.Items {

		functionName, err := mp.getFunctionNameByPodName(pods, podMetric.Name)
		if err != nil {
			continue
		}
		mp.logger.DebugWith("Got function name", "name", functionName, "podName", podMetric.Name)
		for _, container := range podMetric.Containers {
			int64Val := container.Usage.Cpu().MilliValue()

			mp.logger.DebugWith("Container status", "cpu", container.Usage.Cpu())
			newEntry := metricEntry{
				timestamp:    time.Now(),
				value:        int64Val,
				functionName: functionName,
				metricName:   "cpu",
			}
			err := mp.metricReporter.reportMetric(newEntry)
			if err != nil {
				return errors.Wrap(err, "Failed to report metric")
			}
		}
	}
	return nil
}

func (mp *metricsPoller) getFunctionNameByPodName(podList *corev1.PodList, name string) (string, error) {
	var functionName string
	if functionName, found := mp.functionPodNameCache[name]; !found {
		pod, err := mp.getPodByName(podList, name)
		if err != nil {
			return "", err
		}
		functionName = pod.Labels["nuclio.io/function-name"]
		if functionName == "" {
			return "", errors.New("Failed to get function name")
		}
	}

	return functionName, nil
}

func (mp *metricsPoller) getPodByName(podList *corev1.PodList, name string) (*corev1.Pod, error) {
	for _, pod := range podList.Items {
		if pod.Name == name {
			return &pod, nil
		}
	}
	return nil, errors.New("Failed to locate pod in list")
}

func (mp *metricsPoller) start() error {
	go func() {
		for range mp.ticker.C {
			err := mp.getFunctionMetrics()
			if err != nil {
				mp.logger.WarnWith("Failed to get function metrics", "err", err)
			}
			err = mp.getCPUMetrics()
			if err != nil {
				mp.logger.WarnWith("Failed to get cpu metrics")
			}
		}
	}()

	return nil
}
